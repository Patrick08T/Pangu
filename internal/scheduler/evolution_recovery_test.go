//go:build integration

package scheduler

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestEvolutionRootBindsReceiptOnceBeforeDispatchCallback(t *testing.T) {
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.service.PrepareCase(ctx, f.scope, evolution.CaseKey{IntentID: intent.ID, Generation: intent.Generation, Arm: "candidate", CaseID: f.caseID(), Repeat: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = evolutionPersistPlan(ctx, f.scope, *p); err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	begin := func(id string) error {
		return evolutionTransaction(ctx, func(tx pgx.Tx) error { _, e := evolutionBeginCase(ctx, tx, f.scope, *p, id); return e })
	}
	if err = begin(run); err != nil {
		t.Fatalf("worker preceding receipt persistence must bind actual root identity: %v", err)
	}
	if err = begin(uuid.NewString()); !errors.Is(err, evolution.ErrConflict) {
		t.Fatalf("second root stole frozen execution: %v", err)
	}
}
