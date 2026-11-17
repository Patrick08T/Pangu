//go:build integration

package experience

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestStoreDoesNotTurnFailedQueriesIntoDisabledEmptySuccess(t *testing.T) {
	f := newStoreFixture(t)
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
	if _, err = tx.Exec(f.ctx, `SELECT 1/0`); err == nil {
		t.Fatal("expected aborted transaction")
	}
	if _, err = f.store.FreezeTx(f.ctx, tx, f.scope, uuid.NewString(), 1); err == nil {
		t.Fatal("failed query disguised as disabled empty snapshot")
	}
}

func TestStoreCannotActivateOrPublishWithMissingMachineVerifier(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.create(t)
	f.store.VerifyClaim = nil
	if _, err := f.store.Activate(f.ctx, f.scope, version.ID, Activation{ReceiptID: "client-says-passed"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing checker allowed active: %v", err)
	}
	f.store.VerifyBenefit = nil
	if _, err := f.store.SetDelivery(f.ctx, f.scope, version.ID, DeliveryChange{ExpectedRevision: 1,
		Policy: DeliveryPolicy{Mode: "canary", TaskScope: []string{"approved"}, TrafficBasisPoints: 100}, EvidenceID: "client-says-benefit"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing benefit verifier allowed delivery: %v", err)
	}
}
