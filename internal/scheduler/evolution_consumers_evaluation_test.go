//go:build integration

package scheduler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestEvolutionConsumersUsesIndependentEvaluationStopProof(t *testing.T) {
	base, plan, run := evolutionStartedFixture(t, "http://127.0.0.1:1")
	f := evolutionConsumersFixture{evolutionFixture: base, target: evolution.RetentionTarget{IntentID: plan.Key.IntentID, Generation: plan.Key.Generation, ExperimentID: plan.Experiment.ID}}
	if ok, err := f.guard(t, "stopped"); ok || err != nil {
		t.Fatal("known running evaluator treated as unknown/stopped", ok, err)
	}
	if err := evolutionFinishCase(context.Background(), plan.RequestID, run, "failed", nil); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `SELECT pangu.purge_run_payloads($1::uuid)`, run)
	if ok, err := f.guard(t, "stopped"); !ok || err != nil {
		t.Fatal("erased scorer bodies prevented stop proof", ok, err)
	}
}

func TestEvolutionConsumersDetectsActualPurgedBusinessLineage(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	item := f.submit(t, f.target.VersionIDs[0], true, false)
	f.exec(t, `SELECT pangu.purge_run_payloads($1::uuid)`, item.RunID)
	if ok, err := f.guard(t, "stopped"); ok || !errors.Is(err, ErrEvolutionConsumerStopUnknown) {
		t.Fatal("purged role lineage silently omitted", ok, err)
	}
}

func TestEvolutionConsumersDoesNotOmitPreviousEvaluationGeneration(t *testing.T) {
	base, plan, run := evolutionStartedFixture(t, "http://127.0.0.1:1")
	f := evolutionConsumersFixture{evolutionFixture: base, target: evolution.RetentionTarget{IntentID: plan.Key.IntentID, Generation: 2, ExperimentID: plan.Experiment.ID}}
	if err := evolutionFinishCase(context.Background(), plan.RequestID, run, "failed", nil); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `SELECT pangu.purge_run_payloads($1::uuid)`, run)
	f.exec(t, `UPDATE pangu.evolution_intents SET generation=2 WHERE id=$1`, plan.Key.IntentID)
	if ok, err := f.guard(t, "stopped"); ok || !errors.Is(err, ErrEvolutionConsumerStopUnknown) {
		t.Fatal("older generation disappeared from stop proof", ok, err)
	}
}

func TestEvolutionConsumersPurgedEvaluationCannotRestartOrCauseUnrelatedCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { evolutionResponse(w, true) }))
	defer server.Close()
	base, plan, run := evolutionStartedFixture(t, server.URL)
	f := evolutionConsumersFixture{evolutionFixture: base}
	if err := RunEvolutionCase(context.Background(), base.scope.Namespace, plan.RequestID, run); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `SELECT pangu.purge_run_payloads($1::uuid)`, run)
	if _, err := base.service.CancelEvaluation(context.Background(), base.scope, plan.Key.IntentID, 1, "retire fixture"); err != nil {
		t.Fatal(err)
	}
	request := base.request
	request.ID = uuid.NewString()
	intent, err := base.service.ReserveCandidate(context.Background(), base.scope, request)
	if err != nil {
		t.Fatal(err)
	}
	f.target = evolution.RetentionTarget{IntentID: intent.ID, Generation: intent.Generation, ExperimentID: intent.ExperimentID}
	version := f.privateVersion(t, base.request.BaselineVersion, "")
	f.target.VersionIDs = []string{version}
	f.exec(t, `UPDATE pangu.evolution_intents SET publication=$2 WHERE id=$1`, intent.ID, evolutionJSON(evolution.PublishedVersions{AgentID: version}))
	attempt := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: "late"}
	if err = execution.Begin(context.Background(), &attempt); !errors.Is(err, execution.ErrCancelled) {
		t.Fatal("purged evaluator could restart", err)
	}
	f.expire(t)
	out, err := reconcileEvolutionRetention(context.Background(), f.serverScope(), f.target.IntentID, 1, func(context.Context, evolution.Scope, evolutionConsumer) error {
		t.Fatal("unrelated purged evaluator caused cancellation")
		return nil
	})
	if err != nil || out.Deferred || !out.Due {
		t.Fatal("unrelated safely purged evaluation pinned cleanup", out, err)
	}
}

func TestEvolutionConsumersPropagatesSQLFailure(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	ok, err := EvolutionRetentionGuard(context.Background(), tx, f.scope, f.target, "stopped")
	if ok || !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal("SQL failure translated to stop proof or unknown", ok, err)
	}
}
