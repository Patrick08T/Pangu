//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func evolutionPendingFixture(t *testing.T) (evolutionFixture, evolution.ExecutionPlan) {
	t.Helper()
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	return f, evolutionPreparePending(t, f)
}

func evolutionPreparePending(t *testing.T, f evolutionFixture) evolution.ExecutionPlan {
	t.Helper()
	ctx := context.Background()
	i, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.service.PrepareCase(ctx, f.scope, evolution.CaseKey{IntentID: i.ID, Generation: i.Generation, Arm: "candidate", CaseID: f.caseID(), Repeat: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = evolutionPersistPlan(ctx, f.scope, *p); err != nil {
		t.Fatal(err)
	}
	return *p
}

func evolutionDispatchState(t *testing.T, tx pgx.Tx, request string) string {
	t.Helper()
	var state string
	err := tx.QueryRow(context.Background(), `SELECT jsonb_build_object('submission',to_jsonb(s),'execution',to_jsonb(e),'intent',to_jsonb(i),'results',(SELECT count(*) FROM pangu.evolution_machine_results WHERE request_id=s.id),'executions',(SELECT count(*) FROM pangu.evolution_executions WHERE intent_id=i.id),'claims',(SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=i.id),'charges',(SELECT count(*) FROM pangu.evolution_budget_charges WHERE namespace=i.namespace AND project_id=i.project_id),'runtime',(SELECT count(*) FROM pangu.evolution_case_runtime WHERE request_id=s.id))::text FROM pangu.submissions s JOIN pangu.evolution_executions e ON e.request_id=s.id JOIN pangu.evolution_intents i ON i.id=e.intent_id WHERE s.id=$1`, request).Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func evolutionGuardRejects(t *testing.T, f evolutionFixture, p evolution.ExecutionPlan, mutate string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, mutate, p.RequestID); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before := evolutionDispatchState(t, tx, p.RequestID)
	_ = tx.Rollback(ctx)
	calls := 0
	_, err = evolutionDispatch(ctx, f.scope.Namespace, p.RequestID, func(context.Context, string) (string, error) { calls++; return uuid.NewString(), nil })
	if err == nil || calls != 0 {
		t.Fatalf("invalid current request reached RPC: calls=%d error=%v", calls, err)
	}
	tx, err = conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if after := evolutionDispatchState(t, tx, p.RequestID); before != after {
		t.Fatal("rejected dispatch changed durable state")
	}
}

func TestEvolutionDispatchRejectsCurrentSubmissionDespiteStaleCaller(t *testing.T) {
	for name, update := range map[string]string{
		"purged":   `UPDATE pangu.submissions SET purged_at=now() WHERE id=$1`,
		"purpose":  `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{purpose}','"business"') WHERE id=$1`,
		"actor":    `UPDATE pangu.submissions SET created_by=NULL WHERE id=$1`,
		"approval": `UPDATE pangu.submissions SET snapshot=snapshot||'{"approval_origin":"unexpected"}' WHERE id=$1`,
		"fork":     `UPDATE pangu.submissions SET snapshot=snapshot||'{"fork":{}}' WHERE id=$1`,
		"source":   `UPDATE pangu.submissions SET snapshot=snapshot||'{"source_run_id":"unexpected"}' WHERE id=$1`,
		"input":    `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{workflow,input}','"changed current input"') WHERE id=$1`,
	} {
		t.Run(name, func(t *testing.T) { f, p := evolutionPendingFixture(t); evolutionGuardRejects(t, f, p, update) })
	}
}

func TestEvolutionDispatchRejectsCurrentExecutionBeforeRefreshingPlan(t *testing.T) {
	for name, update := range map[string]string{
		"input":      `UPDATE pangu.evolution_executions SET input_hash=repeat('0',64) WHERE request_id=$1`,
		"budget":     `UPDATE pangu.evolution_executions SET max_tokens=max_tokens+1 WHERE request_id=$1`,
		"terminal":   `UPDATE pangu.evolution_executions SET status='failed' WHERE request_id=$1`,
		"generation": `UPDATE pangu.evolution_executions SET generation=generation+1 WHERE request_id=$1`,
		"arm":        `UPDATE pangu.evolution_executions SET arm='baseline' WHERE request_id=$1`,
		"case":       `UPDATE pangu.evolution_executions SET case_id=gen_random_uuid() WHERE request_id=$1`,
		"repeat":     `UPDATE pangu.evolution_executions SET repeat=repeat+1 WHERE request_id=$1`,
		"calls":      `UPDATE pangu.evolution_executions SET max_calls=max_calls+1 WHERE request_id=$1`,
	} {
		t.Run(name, func(t *testing.T) { f, p := evolutionPendingFixture(t); evolutionGuardRejects(t, f, p, update) })
	}
}

func TestEvolutionDispatchBoundRunNeverCallsEngineAgain(t *testing.T) {
	f, p, run := evolutionStartedFixture(t, "http://127.0.0.1:1")
	calls := 0
	got, err := evolutionDispatch(context.Background(), f.scope.Namespace, p.RequestID, func(context.Context, string) (string, error) { calls++; return uuid.NewString(), nil })
	if err != nil || calls != 0 || got != run {
		t.Fatalf("bound run redispatched: %s %s calls=%d %v", run, got, calls, err)
	}
}

func TestEvolutionDispatchCancelledContextDoesNotContactEngine(t *testing.T) {
	f, p := evolutionPendingFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err := evolutionDispatch(ctx, f.scope.Namespace, p.RequestID, func(context.Context, string) (string, error) { calls++; return uuid.NewString(), nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancelled dispatch reached engine: %d %v", calls, err)
	}
}
