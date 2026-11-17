//go:build integration

package experience

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestStoreFreezesEmptyDisabledPolicyAndTransactionRollback(t *testing.T) {
	f := newStoreFixture(t)
	run := uuid.NewString()
	mark, err := f.store.Freeze(f.ctx, f.scope, run, 1)
	if err != nil || len(mark.Versions) != 0 || mark.Hash == "" {
		t.Fatalf("disabled snapshot: %+v %v", mark, err)
	}
	f.policy(t)
	f.serving(t)
	frozen, err := f.store.Freeze(f.ctx, f.scope, run, 1)
	if err != nil || frozen.Hash != mark.Hash || len(frozen.Versions) != 0 {
		t.Fatalf("old snapshot rewritten: %+v %v", frozen, err)
	}
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	tx, err := conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	other := uuid.NewString()
	if _, err = f.store.FreezeTx(f.ctx, tx, f.scope, other, 1); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.ReadWatermark(f.ctx, f.scope, other, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("snapshot escaped submission rollback: %v", err)
	}
}

func TestStoreSelectionReusesFrozenChoiceAndRechecksRevocation(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.serving(t)
	run := uuid.NewString()
	if _, err := f.store.Freeze(f.ctx, f.scope, run, 1); err != nil {
		t.Fatal(err)
	}
	input := SelectionInput{ProjectID: f.scope.ProjectID, RunID: run, NodeID: "node", Round: 1,
		Generation: 1, ApprovalRevision: 1, BoundInputHash: "bound", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	usage, selected, err := f.store.Select(f.ctx, f.scope, input)
	if err != nil || len(selected) != 1 || selected[0].ID != version.ID {
		t.Fatalf("first choice: %+v %v", selected, err)
	}
	input.Text = "unrelated retry text must not rerank"
	if _, err = f.store.SetDelivery(f.ctx, f.scope, version.ID, DeliveryChange{ExpectedRevision: version.DeliveryRevision, Policy: DeliveryPolicy{Mode: "paused"}}); err != nil {
		t.Fatal(err)
	}
	retry, selected, err := f.store.Select(f.ctx, f.scope, input)
	if err != nil || len(selected) != 1 || retry.InjectionHash != usage.InjectionHash {
		t.Fatalf("retry changed: %+v %v", retry, err)
	}
	input.Round++
	_, selected, err = f.store.Select(f.ctx, f.scope, input)
	if err != nil || len(selected) != 0 {
		t.Fatalf("paused fresh choice: %+v %v", selected, err)
	}
	if err := f.store.Revoke(f.ctx, f.scope, version.ID, "permission withdrawn"); err != nil {
		t.Fatal(err)
	}
	input.Round = 1
	if _, _, err := f.store.Select(f.ctx, f.scope, input); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("revoked frozen input survived: %v", err)
	}
}

func TestStoreDependenciesAndIntentRecoveryPersistAcrossInstances(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.serving(t)
	ref := Reference{VersionID: version.ID, BodyHash: version.BodyHash, DeliveryRevision: version.DeliveryRevision}
	upstream := Artifact{ID: "upstream", ProjectID: f.scope.ProjectID, Hash: "out-a", InputHash: "in-a", Kind: "output", ProofVersion: 1, References: []Reference{ref}}
	child := Artifact{ID: "checkpoint", ProjectID: f.scope.ProjectID, Hash: "out-b", InputHash: "in-b", Kind: "checkpoint", ProofVersion: 1, Parents: []ArtifactReference{{ID: "upstream", Hash: "out-a"}}}
	for _, artifact := range []Artifact{upstream, child} {
		if err := f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
			t.Fatal(err)
		}
	}
	root := ArtifactReference{ID: child.ID, Hash: child.Hash}
	if err := f.store.CheckArtifact(f.ctx, f.scope, root); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Revoke(f.ctx, f.scope, version.ID, "source revoked"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.CheckArtifact(f.ctx, f.scope, root); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("transitive revoke not immediate: %v", err)
	}
	input := IntentInput{SourceRunID: uuid.NewString(), EvidenceRevision: 1, PolicyRevision: 1, Purpose: "business", Terminal: true, StopConfirmed: true}
	intent, err := f.store.EnsureLearningIntent(f.ctx, f.scope, input)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := f.store.EnsureLearningIntent(f.ctx, f.scope, input)
	if err != nil || duplicate.ID != intent.ID || duplicate.IdempotencyKey != intent.IdempotencyKey {
		t.Fatalf("duplicate intent: %+v %v", duplicate, err)
	}
	if _, err = f.store.CompleteLearningIntent(f.ctx, f.scope, intent.ID, IntentReceipt{Generation: 0, ReceiptID: "old", Stage: "completed"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale receipt: %v", err)
	}
	if _, err = f.store.CompleteLearningIntent(f.ctx, f.scope, intent.ID, IntentReceipt{Generation: 1, ReceiptID: "failed", Stage: "failed", Retryable: true}); err != nil {
		t.Fatal(err)
	}
	next, err := f.store.RetryLearningIntent(f.ctx, f.scope, intent.ID, 1)
	if err != nil || next.Generation != 2 {
		t.Fatalf("retry persistence: %+v %v", next, err)
	}
	if _, err = f.store.CompleteLearningIntent(f.ctx, f.scope, intent.ID, IntentReceipt{Generation: 1, ReceiptID: "late", Stage: "completed"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("old generation survived retry: %v", err)
	}
}
