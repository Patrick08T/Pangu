//go:build integration

package scheduler

import (
	"context"
	"testing"
)

func TestExperienceErasureReconcileEmptyNamespaceIsNoOp(t *testing.T) {
	_ = newErasureFixture(t, "ttl")
	b := maintenanceBackend(t)
	if err := b.ReconcileExperienceErasure(context.Background()); err != nil {
		t.Fatal("namespace without derived consumers is not an error", err)
	}
}
