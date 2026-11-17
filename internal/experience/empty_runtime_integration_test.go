//go:build integration

package experience

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func anonymousRuntime(f storeFixture) (Store, Scope) {
	scope := f.scope
	scope.ActorID = ""
	store := NewFrozenEmptyStore(func(_ context.Context, _ pgx.Tx, got Scope) error {
		if got.Namespace != f.scope.Namespace || got.ProjectID != f.scope.ProjectID {
			return ErrForbidden
		}
		return nil
	})
	return store, scope
}

func TestStoreFrozenEmptyRuntimeSelectsOriginalEmptyAndWritesEmptyProof(t *testing.T) {
	f := newStoreFixture(t)
	original, err := f.store.Freeze(f.ctx, f.scope, "empty-run", 1)
	if err != nil {
		t.Fatal(err)
	}
	f.policy(t)
	f.serving(t)
	store, scope := anonymousRuntime(f)
	mark, err := store.ReadWatermark(f.ctx, scope, "empty-run", 1)
	if err != nil || mark.Hash != original.Hash {
		t.Fatalf("anonymous frozen read: %+v %v", mark, err)
	}
	input := SelectionInput{ProjectID: scope.ProjectID, RunID: "empty-run", NodeID: "node", Round: 1, Generation: 1, ApprovalRevision: 1, BoundInputHash: "bound", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	usage, versions, err := store.Select(f.ctx, scope, input)
	if err != nil || len(versions) != 0 || usage.WatermarkHash != original.Hash {
		t.Fatalf("empty runtime expanded input: %+v %v", versions, err)
	}
	proof := Artifact{ID: "empty-proof", ProjectID: scope.ProjectID, RunID: "empty-run", Kind: "output", Hash: "output", InputHash: "effective", ProofVersion: 1}
	if err = store.SaveArtifact(f.ctx, scope, proof); err != nil {
		t.Fatal(err)
	}
	if err = store.CheckArtifactInput(f.ctx, scope, ArtifactReference{ID: proof.ID, Hash: proof.Hash}, "effective"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreFrozenEmptyRuntimeRejectsActorsPolicyAndSnapshotCreation(t *testing.T) {
	f := newStoreFixture(t)
	if _, err := f.store.Freeze(f.ctx, f.scope, "empty-run", 1); err != nil {
		t.Fatal(err)
	}
	store, scope := anonymousRuntime(f)
	if _, err := store.ReadWatermark(f.ctx, f.scope, "empty-run", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("identified actor used anonymous store: %v", err)
	}
	if _, err := store.SavePolicy(f.ctx, scope, Policy{}, 0); !errors.Is(err, ErrForbidden) {
		t.Fatalf("anonymous store wrote policy: %v", err)
	}
	if _, err := store.Freeze(f.ctx, scope, "new-run", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("anonymous store froze new task: %v", err)
	}
	ordinary := Store{Authorize: store.Authorize}
	if _, err := ordinary.ReadWatermark(f.ctx, scope, "empty-run", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ordinary store actor guard weakened: %v", err)
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
	defer tx.Rollback(f.ctx)
	if _, err = store.FreezeEmptyTx(f.ctx, tx, scope, "new-run", 1, "business"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("anonymous runtime created empty watermark: %v", err)
	}
}

func TestStoreFrozenEmptyRuntimeRejectsNonemptyWatermarksAndAncestors(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.serving(t)
	if _, err := f.store.Freeze(f.ctx, f.scope, "nonempty-run", 1); err != nil {
		t.Fatal(err)
	}
	parent := Artifact{ID: "parent", ProjectID: f.scope.ProjectID, Hash: "parent-hash", InputHash: "input", ProofVersion: 1, References: []Reference{versionReference(version)}}
	child := Artifact{ID: "child", ProjectID: f.scope.ProjectID, Hash: "child-hash", InputHash: "input", ProofVersion: 1, Parents: []ArtifactReference{{ID: parent.ID, Hash: parent.Hash}}}
	for _, proof := range []Artifact{parent, child} {
		if err := f.store.SaveArtifact(f.ctx, f.scope, proof); err != nil {
			t.Fatal(err)
		}
	}
	store, scope := anonymousRuntime(f)
	sourceReads := 0
	store.VerifySource = func(context.Context, pgx.Tx, Scope, Source) error { sourceReads++; return nil }
	if _, err := store.ReadWatermark(f.ctx, scope, "nonempty-run", 1); err == nil {
		t.Fatal("nonempty watermark returned")
	}
	input := SelectionInput{ProjectID: scope.ProjectID, RunID: "nonempty-run", NodeID: "node", Round: 1, Generation: 1, ApprovalRevision: 1, BoundInputHash: "bound", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	if _, _, err := store.Select(f.ctx, scope, input); err == nil {
		t.Fatal("nonempty selection admitted")
	}
	if err := store.ValidateReferences(f.ctx, scope, []Reference{versionReference(version)}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("nonempty references accepted: %v", err)
	}
	if err := store.CheckArtifact(f.ctx, scope, ArtifactReference{ID: child.ID, Hash: child.Hash}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ancestor memory accepted: %v", err)
	}
	if sourceReads != 0 {
		t.Fatalf("anonymous runtime read %d private sources", sourceReads)
	}
}
