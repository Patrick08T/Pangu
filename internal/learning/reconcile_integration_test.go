//go:build integration

package learning

import (
	"context"
	"testing"
)

func TestMemoryReconcilePersistsPauseWithoutChangingValidityOrBody(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	paused, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, nil)
	if err != nil || !paused {
		t.Fatal("missing trusted evidence did not persist pause", paused, err)
	}
	current, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil || current.Delivery != "paused" || current.Validity != "active" || current.BodyHash != version.BodyHash {
		t.Fatal("pause changed claim validity/body", current, err)
	}
	again, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, nil)
	if err != nil || again {
		t.Fatal("pause reconciliation not idempotent", again, err)
	}
}

func TestMemoryReconcileCannotUseRemovedPolicyActor(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	f.exec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1 AND project_id=$2`, f.actor.ID, f.scope.ProjectID)
	if _, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, nil); err == nil {
		t.Fatal("removed policy author mutated delivery")
	}
}

func TestLearningLineageReadRequiresRealCurrentMembership(t *testing.T) {
	f := newLearningFixture(t)
	tx := f.transaction(t)
	scope := f.scope
	scope.Action = "lineage_read"
	if err := authorize(f.ctx, tx, scope); err != nil {
		t.Fatal("lineage read rejected", err)
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1 AND project_id=$2`, f.actor.ID, f.scope.ProjectID)
	tx = f.transaction(t)
	if err := authorize(f.ctx, tx, scope); err == nil {
		t.Fatal("removed member read lineage")
	}
	_ = tx.Rollback(f.ctx)
}
