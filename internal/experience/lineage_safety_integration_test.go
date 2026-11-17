//go:build integration

package experience

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestRunLineageRedactsWithdrawnSourcesButNotInfrastructureErrors(t *testing.T) {
	f := lineageFixture(t)
	lineageRecords(t, f)
	f.store.VerifySource = func(context.Context, pgx.Tx, Scope, Source) error { return ErrForbidden }
	got, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery())
	if err != nil || got.Watermark.Status != "restricted" || got.Watermark.Hash != "" || len(got.Watermark.References) != 0 {
		t.Fatalf("source scope leaked: %+v %v", got, err)
	}
	if len(got.Usages.Items) != 1 || got.Usages.Items[0].InjectionHash != "" || !got.Usages.Items[0].Redacted {
		t.Fatalf("usage hashes leaked: %+v", got.Usages)
	}
	child, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "b-child")
	if err != nil || child.Status != "restricted" || child.OutputHash != "" || child.InputHash != "" || child.ExecutionInputHash != "" || child.Parents[0].Hash != "" {
		t.Fatalf("ancestor leaked through child: %+v %v", child, err)
	}
	unavailable := errors.New("source storage unavailable")
	f.store.VerifySource = func(context.Context, pgx.Tx, Scope, Source) error { return unavailable }
	if _, err = f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery()); !errors.Is(err, unavailable) {
		t.Fatalf("infra disguised as audit invalidation: %v", err)
	}
}

func TestRunLineageRejectsRevokedActorAndAnonymousCapability(t *testing.T) {
	f := lineageFixture(t)
	f.store.Authorize = func(context.Context, pgx.Tx, Scope) error { return ErrForbidden }
	if _, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("actor withdrawal ignored: %v", err)
	}
	if _, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "unknown"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("actor can read audit: %v", err)
	}
	store := NewFrozenEmptyStore(func(context.Context, pgx.Tx, Scope) error { return nil })
	store.VerifyRun = f.store.VerifyRun
	f.scope.ActorID = ""
	if _, err := store.ReadRunLineage(f.ctx, f.scope, lineageQuery()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("runtime capability used for audit: %v", err)
	}
}

func TestRunLineageMarksLegacyArtifactUnproven(t *testing.T) {
	f := lineageFixture(t)
	artifact := Artifact{ID: "legacy", RunID: "actual-run", ProjectID: f.scope.ProjectID, Kind: "output", Hash: "old-output", InputHash: "logical", ProofVersion: 1}
	if err := f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.ReadArtifactSummary(f.ctx, f.scope, artifact.ID)
	if err != nil || got.Status != "unproven" || got.Proven || got.InvalidReason != "execution_input_missing" {
		t.Fatalf("legacy proof called reusable: %+v %v", got, err)
	}
}

func TestRunLineageVerifierUsesAuthorizedTransaction(t *testing.T) {
	f := lineageFixture(t)
	authorize := f.store.Authorize
	verify := f.store.VerifyRun
	var authorized pgx.Tx
	checks := 0
	f.store.Authorize = func(ctx context.Context, tx pgx.Tx, scope Scope) error {
		authorized = tx
		checks++
		return authorize(ctx, tx, scope)
	}
	called := false
	f.store.VerifyRun = func(ctx context.Context, tx pgx.Tx, scope Scope, q RunLineageQuery) (RunBinding, error) {
		if tx != authorized || scope.Action != "lineage_read" {
			t.Fatal("verifier outside authorized transaction")
		}
		called = true
		return verify(ctx, tx, scope, q)
	}
	if _, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery()); err != nil {
		t.Fatal(err)
	}
	if !called || checks < 2 {
		t.Fatalf("binding and precommit checks omitted: %v %d", called, checks)
	}
}
