package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestExperimentTxRejectsInvalidArmBeforeDB(t *testing.T) {
	_, _, err := CompileExperimentTx(context.Background(), nil, "namespace", "experiment", "forged", "input")
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("非法实验臂被接受", err)
	}
}

func TestExperimentTxCompilationAndRollbackKeepOuterTransaction(t *testing.T) {
	f := newEvolutionFixture(t)
	ctx := context.Background()
	payload := CandidatePayload{Agent: *f.baseline.Spec.Agent}
	payload.Agent.Instruction = "受控候选"
	prepared, err := PrepareExperimentCandidate(ctx, f.namespace, f.project, f.baseline.ID, payload)
	if err != nil {
		t.Fatal(err)
	}
	experiment := seedCatalogExperiment(t, f.namespace, f.project, f.baseline.ID, prepared)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	p, err := PrepareExperimentCandidateTx(ctx, tx, f.namespace, f.project, f.baseline.ID, payload)
	if err != nil || p.CandidateCompiledHash != prepared.CandidateCompiledHash {
		t.Fatal(p, err)
	}
	if err = StoreExperimentCandidateTx(ctx, tx, f.namespace, experiment, payload); err != nil {
		t.Fatal(err)
	}
	d, identity, err := CompileExperimentTx(ctx, tx, f.namespace, experiment, "candidate", "case input")
	if err != nil || identity.ID != experiment || d.Input != "case input" {
		t.Fatal(d, identity, err)
	}
	marker := uuid.NewString()
	if _, err = tx.Exec(ctx, `SELECT $1::uuid`, marker); err != nil {
		t.Fatal("Tx被内部提交", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err = CompileExperiment(ctx, f.namespace, experiment, "candidate", "case input"); !errors.Is(err, ErrNotFound) {
		t.Fatal("回滚未撤销候选", err)
	}
}
