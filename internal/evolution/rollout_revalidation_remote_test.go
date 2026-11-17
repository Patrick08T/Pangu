//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"
)

func rolloutRevalidationExternal(t *testing.T) (rolloutFixture, rolloutEntries) {
	t.Helper()
	f, adapter := rolloutWithExternal(t)
	ctx := context.Background()
	if _, err := f.service.Reconcile(ctx, f.scope, f.id, 1); err != nil {
		t.Fatal(err)
	}
	rolloutRemote(t, f, adapter, "confirmed")
	intent, err := f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "observing" {
		t.Fatal("fixture could not begin confirmed observation", intent, err)
	}
	return f, adapter
}

func TestRolloutRevalidationInvalidSourceKeepsRemoteClaims(t *testing.T) {
	f, adapter := rolloutRevalidationExternal(t)
	ctx := context.Background()
	cohort := rolloutRevalidationSeed(t, &f, "workbench:main", 1)
	rolloutRevalidationPut(t, f, cohort[0].RunID, rolloutRevalidationSource{Evidence: cohort[0], Problem: "deleted"})
	rolloutRemote(t, f, adapter, "unknown")
	intent, err := f.service.Advance(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "rolling_back" {
		t.Fatal("invalid source did not start compensation", intent, err)
	}
	rolloutRevalidationState(t, f, "rolling_back", "revoked", 3, 3)
	intent, err = f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "reconciling" {
		t.Fatal("unknown remote did not retain recovery", intent, err)
	}
	_, calls := rolloutRemote(t, f, adapter, "confirmed")
	if calls != 1 {
		t.Fatal("unknown remote was blindly changed", calls)
	}
	rolloutRevalidationState(t, f, "reconciling", "revoked", 3, 3)
	intent, err = f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "reconciling" {
		t.Fatal("pending compensation released ownership", intent, err)
	}
	remote, calls := rolloutRemote(t, f, adapter, "confirmed")
	if remote.VersionID != f.request.BaselineVersion || calls != 2 {
		t.Fatal("baseline compensation missing", remote, calls)
	}
	rolloutRevalidationState(t, f, "reconciling", "revoked", 3, 3)
	intent, err = f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "insufficient" {
		t.Fatal("confirmed compensation not finalized", intent, err)
	}
	rolloutRevalidationState(t, f, "insufficient", "revoked", 3, 0)
}

func TestRolloutRevalidationChecksWholeCohortAcrossEntries(t *testing.T) {
	f, _ := rolloutRevalidationExternal(t)
	one := rolloutRevalidationSeed(t, &f, "workbench:main", 0)
	two := rolloutRevalidationSeed(t, &f, "cron:daily", 1)
	rolloutRevalidationPut(t, f, one[0].RunID, rolloutRevalidationSource{Evidence: one[0], Problem: "deleted"})
	rolloutRevalidationPut(t, f, two[2].RunID, rolloutRevalidationSource{Evidence: two[2], Problem: "deadline"})
	if _, err := f.service.Advance(context.Background(), f.scope, f.id, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("first entry decision skipped unresolved evidence in another entry", err)
	}
	rolloutRevalidationState(t, f, "observing", "observing", 6, 3)
}
