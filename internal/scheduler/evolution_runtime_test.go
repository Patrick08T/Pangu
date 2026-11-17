//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func TestEvolutionRuntimeExecutesRealFullFlowAndKeepsCheckerHidden(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "checker-only-value") || strings.Contains(string(body), "json_equal") || strings.Contains(string(body), "candidate_hash") {
			t.Error("checker or experiment metadata reached model")
		}
		text := `{"draft":true}`
		if calls.Add(1)%2 == 0 {
			text = `{"answer":"checker-only-value"}`
			if !strings.Contains(string(body), "draft-task") && !strings.Contains(string(body), "draft") {
				t.Error("lost upstream flow result")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "fixture", "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}})
	}))
	defer server.Close()
	f := newEvolutionFixture(t, server.URL)
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	key := evolution.CaseKey{IntentID: intent.ID, Generation: 1, Arm: "candidate", CaseID: f.caseID(), Repeat: 1}
	plan, err := f.service.PrepareCase(ctx, f.scope, key)
	if err != nil {
		t.Fatal(err)
	}
	if err = evolutionPersistPlan(ctx, f.scope, *plan); err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, f.scope.Namespace, plan.RequestID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	if err = RunEvolutionCase(ctx, f.scope.Namespace, plan.RequestID, run); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("not actual fullFlow execution: calls=%d", calls.Load())
	}
	if err = RunEvolutionCase(ctx, f.scope.Namespace, plan.RequestID, run); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("completed recovery repeated model calls")
	}
	f.service.Evidence = EvolutionEvidence{}
	if err = f.service.RecordCaseResult(ctx, f.scope, key, plan.RequestID); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var count int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.usage_calls WHERE run_id=$1 AND input_tokens=10 AND output_tokens=5 AND estimate_micros IS NOT NULL`, run).Scan(&count); err != nil || count != 2 {
		t.Fatal("usage did not originate in provider transport", count, err)
	}
}
