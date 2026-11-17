//go:build integration

package learning

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestRetentionExpiryPhysicallyErasesOnlyExpiredScope(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	expired, expiredChecker := f.retentionExpiredCopy(t, version, checker)
	other := newLearningFixture(t)
	otherVersion, _, otherChecker := other.claim(t)
	untouched, untouchedChecker := other.retentionExpiredCopy(t, otherVersion, otherChecker)
	tx := f.transaction(t)
	out, err := PurgeExpiredTx(context.Background(), tx, f.scope.Namespace, f.scope.ProjectID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if out.Bodies != 1 || out.CheckerBodies != 1 {
		t.Fatalf("expiry did not erase exact private rows: %+v", out)
	}
	if a, b := f.retentionCounts(t, expired, expiredChecker); a != 0 || b != 0 {
		t.Fatal("expired payload physically retained", a, b)
	}
	if a, b := f.retentionCounts(t, version.ID, checker.ID); a != 1 || b != 1 {
		t.Fatal("unexpired payload erased", a, b)
	}
	if a, b := other.retentionCounts(t, untouched, untouchedChecker); a != 1 || b != 1 {
		t.Fatal("cross-project expiry erased", a, b)
	}
}

func TestRetentionMaintenanceRejectsScopeAndBounds(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	expired, expiredChecker := f.retentionExpiredCopy(t, version, checker)
	for _, limit := range []int{0, -1, 1001} {
		tx := f.transaction(t)
		if _, err := PurgeExpiredTx(f.ctx, tx, f.scope.Namespace, f.scope.ProjectID, limit); err == nil {
			t.Fatal("invalid retention bound accepted", limit)
		}
		tx.Rollback(f.ctx)
	}
	tx := f.transaction(t)
	if _, err := PurgeExpiredTx(f.ctx, tx, "another-namespace", f.scope.ProjectID, 1); err == nil {
		t.Fatal("namespace-project mismatch accepted")
	}
	tx.Rollback(f.ctx)
	if a, b := f.retentionCounts(t, expired, expiredChecker); a != 1 || b != 1 {
		t.Fatal("invalid sweep erased rows", a, b)
	}
}

func TestRetentionMaintenanceRollsBackAndIgnoresDisabledPolicy(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	expired, expiredChecker := f.retentionExpiredCopy(t, version, checker)
	if _, err := f.store.SavePolicy(f.ctx, f.scope, experience.Policy{}, 1); err != nil {
		t.Fatal(err)
	}
	tx := f.transaction(t)
	if _, err := PurgeExpiredTx(context.Background(), tx, f.scope.Namespace, f.scope.ProjectID, 10); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	if a, b := f.retentionCounts(t, expired, expiredChecker); a != 1 || b != 1 {
		t.Fatal("rollback lost payload", a, b)
	}
	tx = f.transaction(t)
	out, err := PurgeExpiredTx(context.Background(), tx, f.scope.Namespace, f.scope.ProjectID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Bodies != 1 || out.CheckerBodies != 1 {
		t.Fatal("disabled policy blocked obligatory erasure", out)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
}
