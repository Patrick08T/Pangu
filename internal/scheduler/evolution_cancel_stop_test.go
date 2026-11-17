//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestEvolutionCancelStopMissingRuntimeAndStorageFailuresRemainUnknown(t *testing.T) {
	f, plan, _ := evolutionStartedFixture(t, "http://127.0.0.1:1")
	ctx := context.Background()
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
	reader := EvolutionEvidence{}
	if _, err = reader.ReadStopTx(ctx, tx, f.scope, uuid.NewString()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("missing runtime became stop", err)
	}
	if _, err = reader.ReadStopTx(ctx, tx, f.scope, plan.RequestID); !errors.Is(err, evolution.ErrChecker) {
		t.Fatal("running runtime became stop", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = reader.ReadStopTx(ctx, tx, f.scope, plan.RequestID); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal("database failure became stop", err)
	}
}

func TestEvolutionCancelStopProofSurvivesAuthorizedBodyErasure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { evolutionResponse(w, true) }))
	defer server.Close()
	f, plan, run := evolutionStartedFixture(t, server.URL)
	ctx := context.Background()
	if err := RunEvolutionCase(ctx, f.scope.Namespace, plan.RequestID, run); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `SELECT pangu.purge_run_payloads($1::uuid)`, run); err != nil {
		t.Fatal(err)
	}
	var proof evolution.StopProof
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if _, e := (EvolutionEvidence{}).ReadExecutionTx(ctx, tx, f.scope, plan.RequestID); e == nil {
			t.Fatal("erased scoring body still readable")
		}
		var e error
		proof, e = (EvolutionEvidence{}).ReadStopTx(ctx, tx, f.scope, plan.RequestID)
		return e
	})
	if err != nil || !proof.StopConfirmed || proof.RunID != run || proof.RequestID != plan.RequestID {
		t.Fatal("stopped runtime required erased body", proof, err)
	}
	out, err := f.service.CancelEvaluation(ctx, f.scope, plan.Key.IntentID, 1, "authorized source erasure")
	if err != nil || out.State != "insufficient" {
		t.Fatal("erased body pinned cancellation", out, err)
	}
	var raw []byte
	var claims int
	if err = conn.QueryRow(ctx, `SELECT result FROM pangu.evolution_machine_results WHERE request_id=$1`, plan.RequestID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var score evolution.MachineResult
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &score) != nil || score.StopConfirmed || score.CompiledHash != "" || json.Unmarshal(raw, &envelope) != nil || len(envelope) != 1 || envelope["stop_proof"] == nil {
		t.Fatal("stop-only evidence became score receipt", string(raw))
	}
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1`, plan.Key.IntentID).Scan(&claims); err != nil || claims != 0 {
		t.Fatal("confirmed stop retained claims", claims, err)
	}
}
