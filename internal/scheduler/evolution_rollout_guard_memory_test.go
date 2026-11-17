//go:build integration

package scheduler

import (
	"context"
	"testing"
)

func TestEvolutionRolloutGuardAuthorizedFrozenMemoryCanDispatchAndConfirm(t *testing.T) {
	f, _, expected, key := rolloutGuardMemoryFixture(t, true)
	if len(f.request.MemoryVersionIDs) != 1 {
		t.Fatal("fixture omitted real frozen memory")
	}
	if err := rolloutGuardDispatch(context.Background(), f, key); err != nil {
		t.Fatal("valid frozen memory blocked remote dispatch", err)
	}
	if err := rolloutGuardReceipt(context.Background(), f, key, expected); err != nil {
		t.Fatal("valid frozen memory blocked remote receipt", err)
	}
}

func TestEvolutionRolloutGuardFrozenMemoryWithdrawalBlocksReceipt(t *testing.T) {
	f, _, expected, key := rolloutGuardMemoryFixture(t, true)
	if err := rolloutGuardDispatch(context.Background(), f, key); err != nil {
		t.Fatal("valid frozen memory blocked remote dispatch", err)
	}
	rolloutGuardExec(t, `UPDATE pangu.learning_source_consents SET revoked_at=clock_timestamp() WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID)
	if err := rolloutGuardReceipt(context.Background(), f, key, expected); err == nil {
		t.Fatal("withdrawn frozen memory authorized remote receipt")
	}
	rolloutGuardRetained(t, f, expected, key)
}
