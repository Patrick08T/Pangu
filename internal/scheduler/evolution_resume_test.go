//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func evolutionStartedFixture(t *testing.T, url string) (evolutionFixture, evolution.ExecutionPlan, string) {
	t.Helper()
	f := newEvolutionFixture(t, url)
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
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error { _, e := evolutionBeginCase(ctx, tx, f.scope, *p, run); return e })
	if err != nil {
		t.Fatal(err)
	}
	return f, *p, run
}

func evolutionResponse(w http.ResponseWriter, measured bool) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{"id": "fixture", "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": `{"answer":"checker-only-value"}`}, "finish_reason": "stop"}}}
	if measured {
		response["usage"] = map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}
	_ = json.NewEncoder(w).Encode(response)
}

func TestEvolutionRecoveryReusesCompletedNodeWithoutRepeatingProvider(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); evolutionResponse(w, true) }))
	defer server.Close()
	f, p, run := evolutionStartedFixture(t, server.URL)
	ctx := context.Background()
	input := workflow.NodeInput{Node: p.Definition.Nodes[0], ProjectID: p.Definition.ProjectID, Input: p.Definition.Input, Parents: map[string]workflow.NodeOutput{}}
	if _, err := evolutionExecuteNode(ctx, f.scope, p, input, run); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("first actual node not persisted")
	}
	if err := RunEvolutionCase(ctx, f.scope.Namespace, p.RequestID, run); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("resume repeated a completed call: %d", calls.Load())
	}
}

func TestEvolutionUnknownUsageStopsNextNodeAndCannotPass(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); evolutionResponse(w, false) }))
	defer server.Close()
	f, p, run := evolutionStartedFixture(t, server.URL)
	ctx := context.Background()
	if err := RunEvolutionCase(ctx, f.scope.Namespace, p.RequestID, run); err == nil {
		t.Fatal("unknown usage was accepted")
	}
	if calls.Load() != 1 {
		t.Fatalf("unknown usage did not stop next node: %d", calls.Load())
	}
	if err := f.service.RecordCaseResult(ctx, f.scope, p.Key, p.RequestID); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var status string
	if err = conn.QueryRow(ctx, `SELECT result->>'Status' FROM pangu.evolution_machine_results WHERE request_id=$1`, p.RequestID).Scan(&status); err != nil || status == "completed" {
		t.Fatalf("unknown usage produced success: %s %v", status, err)
	}
}

func TestEvolutionUncertainNodeIsNeverAutomaticallyRepeated(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); evolutionResponse(w, true) }))
	defer server.Close()
	f, p, run := evolutionStartedFixture(t, server.URL)
	ctx := context.Background()
	input, err := evolutionNodeInput(p, workflow.NodeInput{Node: p.Definition.Nodes[0], ProjectID: p.Definition.ProjectID, Input: p.Definition.Input, Parents: map[string]workflow.NodeOutput{}}, run, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error { _, _, e := evolutionBeginNode(ctx, tx, p, input); return e })
	if err != nil {
		t.Fatal(err)
	}
	if err = RunEvolutionCase(ctx, f.scope.Namespace, p.RequestID, run); err == nil {
		t.Fatal("uncertain attempt was repeated")
	}
	if calls.Load() != 0 {
		t.Fatal("uncertain node caused an extra provider call")
	}
}
