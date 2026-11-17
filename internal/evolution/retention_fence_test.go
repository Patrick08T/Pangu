//go:build integration

package evolution

import (
	"context"
	"testing"
)

func TestRetentionOrdinaryReconcileCannotBypassPendingStopFence(t *testing.T) {
	base := newRolloutFixture(t)
	if _, err := base.service.Reconcile(context.Background(), base.scope, base.id, 1); err != nil {
		t.Fatal(err)
	}
	f := attachRetentionFixture(t, base, "withdrawn")
	pending, err := f.reconcile(t, nil)
	if err != nil || !pending.Deferred {
		t.Fatal(pending, err)
	}
	current, err := f.service.Reconcile(context.Background(), f.scope, f.id, 1)
	if err != nil {
		t.Fatal(err)
	}
	c, v, _, claims := f.retained(t)
	if !c || !v || claims != 2 || current.State != "reconciling" {
		t.Fatal("ordinary rollback bypassed pending consumer stop", c, v, claims, current)
	}
	out, err := f.reconcile(t, retentionAllowed)
	if err != nil || out.Deferred {
		t.Fatal("confirmed consumers did not unblock compensation", out, err)
	}
	c, v, _, claims = f.retained(t)
	if c || v || claims != 0 {
		t.Fatal("confirmed retention incomplete", c, v, claims)
	}
}
