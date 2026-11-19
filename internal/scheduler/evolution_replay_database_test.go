//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestEvolutionReplayNativeReceiptAndCurrentEntry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(replayModel(t, &calls))
	defer server.Close()
	f := replayFixture(t, server.URL)
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	assertCurrentReplayEntry(t, f)
	for _, arm := range []string{"baseline", "candidate"} {
		key := evolution.CaseKey{IntentID: intent.ID, Generation: 1, Arm: arm, CaseID: f.caseID(), Repeat: 1}
		receipt := replayReceipt(t, f, key, intent.ExperimentID)
		want := arm == "baseline"
		if receipt.Status != "completed" || receipt.CheckerHash != evolution.ResearchReplayCheckerHash() ||
			!receipt.GatesPassed || receipt.ProtectionPassed != want || (receipt.Metrics["acceptance"] == 1) != want {
			t.Fatalf("unexpected %s receipt: %+v", arm, receipt)
		}
	}
	if calls.Load() != 4 {
		t.Fatalf("only collector should call the model twice per arm, got %d", calls.Load())
	}
}

func TestEvolutionReplayStopsBeforeNextCallAfterTokenBudget(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(replayModel(t, &calls))
	defer server.Close()
	f := replayFixture(t, server.URL)
	f.request.Budget.Tokens = 40
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	key := evolution.CaseKey{IntentID: intent.ID, Generation: 1, Arm: "baseline", CaseID: f.caseID(), Repeat: 1}
	plan, run := dispatchReplay(t, f, key)
	err = RunEvolutionCase(ctx, f.scope.Namespace, plan.RequestID, run)
	if err == nil || calls.Load() != 1 {
		t.Fatalf("expected token budget to stop the second provider call, err=%v calls=%d", err, calls.Load())
	}
}

func replayModel(t *testing.T, calls *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		n := calls.Add(1)
		if r.URL.Path != "/responses" || !strings.Contains(string(body), `"effort":"high"`) {
			t.Error("回放没有使用指定的 Responses API 和推理强度")
		}
		if strings.Contains(string(body), "never-execute-delivery") || strings.Contains(string(body), "research_replay_v1") {
			t.Error("模型收到了交付指令或评分配置")
		}
		name, args := "read_project", `{"document_index":0}`
		if n%2 == 1 && strings.Contains(string(body), "冻结报告原文") {
			t.Error("冻结原文在模型调用读取工具之前泄露")
		}
		if n%2 == 0 {
			if !strings.Contains(string(body), "冻结报告原文") {
				t.Error("模型未收到冻结原文")
			}
			date := "2026-01-01"
			if n > 2 {
				date = "2026-01-02"
			}
			name, args = "register_report", replayReport(date)
		}
		tool := map[string]any{"call_id": fmt.Sprintf("call_%d", n), "type": "function_call", "name": name, "arguments": args}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{tool}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 5}})
	}
}

func replayReport(date string) string {
	return fmt.Sprintf(`{"title":"回放报告","markdown":"## 结论\n收入增长 12%% [d0]\n## 证据与推断\n见原文 [d0]\n## 未知项\n无\n## 调研范围与方法\n核对冻结资料\n## 发现与建议\n关注收入","source_ids":["d0"],"claims":[{"id":"fact1","kind":"fact","statement":"收入增长 12%%","scope":"本报告","uncertainty":"仅代表本期","evidence":[{"source_id":"d0","quote":"收入增长 12%%","source_date":%q}]}]}`, date)
}

func replayFixture(t *testing.T, url string) evolutionFixture {
	t.Helper()
	ctx := context.Background()
	f := emptyEvolutionFixture(t)
	connection := f.publish(t, "connection", "model", catalog.Spec{Connection: &catalog.Connection{Provider: "openai", API: "responses", ReasoningEffort: "high", Model: "fixture", BaseURL: url, SecretRef: "PANGU_SECRET_EVOLUTION_DRIVER_TEST", Price: &workflow.Price{Currency: "USD", InputPerMillionMicros: 2000000, OutputPerMillionMicros: 10000000}}})
	env, err := f.service.RegisterEnvironment(ctx, f.scope, evolution.Environment{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Backend: evolution.ResearchReplayBackend, ConnectionVersion: connection.ID, Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	handoff := &workflow.HandoffContract{Objective: "调研", Inputs: "冻结资料", Deliverables: "报告", Acceptance: "引用原文", FailureHandling: "记录缺失"}
	node := workflow.Node{ID: "collect", Executor: "eino", Model: "fixture", ConnectionVersion: connection.ID, Role: "product_manager", Mode: "research", Policy: &workflow.Policy{Retries: 0}, Instruction: "整理冻结资料", Handoff: handoff}
	role := f.publish(t, "agent", "collect", catalog.Spec{Agent: &node})
	delivery := f.publish(t, "agent", "delivery", catalog.Spec{Agent: &workflow.Node{ID: "delivery", Executor: "eino", Model: "fixture", ConnectionVersion: connection.ID, Instruction: "never-execute-delivery", Handoff: handoff}})
	flow := f.publish(t, "workflow", "brief", catalog.Spec{Flow: &catalog.Flow{Nodes: []catalog.Binding{
		{ID: "collect", AgentVersion: role.ID, Task: "整理"}, {ID: "feishu", AgentVersion: delivery.ID, Task: "交付", DependsOn: []string{"collect"}}, {ID: "wechat", AgentVersion: delivery.ID, Task: "交付", DependsOn: []string{"collect"}},
	}}})
	f.request = evolution.CandidateRequest{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, PolicyRevision: 1, AgentObjectID: role.ObjectID, FlowObjectID: flow.ObjectID, BaselineVersion: role.ID, EnvironmentID: env.ID, Entries: []evolution.EntryTarget{{Kind: "workbench", ID: "main"}}, Budget: experience.Budget{Calls: 16, Tokens: 4000}, Payload: catalog.CandidatePayload{Agent: node, FlowVersion: flow.ID, BindingIDs: []string{"collect"}}}
	f.request.Payload.Agent.Instruction = "核对日期及原文"
	f.suite = f.createSuiteWithCases(t, "research_replay_v1", evolution.ResearchReplayCheckerHash(), func(i int) (string, string) {
		return fmt.Sprintf(`{"brief":{"question":"整理样例 %d","documents":["release.txt"]},"documents":["冻结报告原文：收入增长 12%%。"]}`, i),
			`{"facts":[{"statement":"收入增长 12%","source_id":"d0","quote":"收入增长 12%","source_date":"2026-01-01"}]}`
	})
	f.configurePolicy(t)
	return f
}

func replayReceipt(t *testing.T, f evolutionFixture, key evolution.CaseKey, experiment string) evaluation.ExperimentReceipt {
	t.Helper()
	ctx := context.Background()
	plan, run := dispatchReplay(t, f, key)
	if err := RunEvolutionCase(ctx, f.scope.Namespace, plan.RequestID, run); err != nil {
		t.Fatal(err)
	}
	if err := f.service.RecordCaseResult(ctx, f.scope, key, plan.RequestID); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var receipt evaluation.ExperimentReceipt
	err = conn.QueryRow(ctx, `SELECT receipt FROM pangu.evaluation_experiment_receipts WHERE experiment_id=$1 AND arm=$2 AND case_id=$3`, experiment, key.Arm, key.CaseID).Scan(&receipt)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func dispatchReplay(t *testing.T, f evolutionFixture, key evolution.CaseKey) (*evolution.ExecutionPlan, string) {
	t.Helper()
	ctx := context.Background()
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
	return plan, run
}

func assertCurrentReplayEntry(t *testing.T, f evolutionFixture) {
	t.Helper()
	ctx := context.Background()
	record := &submission.Record{Operation: "entry", ProjectID: f.scope.ProjectID, Snapshot: submission.Snapshot{Purpose: "business", EntryKey: "workbench:main", EntryRevision: 1, Workflow: workflow.Definition{ProjectID: f.scope.ProjectID, VersionID: f.request.Payload.FlowVersion}}}
	entry, err := runEntry(ctx, f.scope.Namespace, record)
	if err != nil || entry == nil || !entry.Current || entry.WorkflowID != f.request.FlowObjectID {
		t.Fatalf("server-selected entry was not matched: %+v %v", entry, err)
	}
	record.Snapshot.Workflow.VersionID = f.request.BaselineVersion
	entry, err = runEntry(ctx, f.scope.Namespace, record)
	if err != nil || entry == nil || entry.Current {
		t.Fatalf("unbound version was admitted: %+v %v", entry, err)
	}
	record.Operation = "submit"
	entry, err = runEntry(ctx, f.scope.Namespace, record)
	if err != nil || entry != nil {
		t.Fatalf("ordinary submission obtained entry provenance: %+v %v", entry, err)
	}
}
