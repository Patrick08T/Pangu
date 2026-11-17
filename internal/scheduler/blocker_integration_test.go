package scheduler_test

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"sync"
	"testing"
)

func evidenceDefinition(project string) workflow.Definition {
	summary := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"iteration_complete": {Type: "boolean"}, "summary": {Type: "string"}}, Required: []string{"iteration_complete", "summary"}}
	report := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"iteration_complete": {Type: "boolean"}, "summary": {Type: "string"}, "evidence_blockers": workflow.EvidenceBlockersShape()}, Required: []string{"iteration_complete", "summary", "evidence_blockers"}}
	return workflow.Definition{ProjectID: project, Input: "补证验证", Nodes: []workflow.Node{{ID: "plan", Executor: "codex", ReadOnly: true, Instruction: "规划", Handoff: testHandoff()}, {ID: "qa", Executor: "codex", ReadOnly: true, Instruction: "核验", Handoff: testHandoff(), DependsOn: []string{"plan"}, OutputSchema: summary, ReportSchema: report}, {ID: "final", Executor: "codex", ReadOnly: true, Instruction: "独立验收", Handoff: testHandoff(), DependsOn: []string{"qa"}}}}
}

func evidenceOutput(blocked bool) workflow.NodeOutput {
	items := []workflow.EvidenceBlocker{}
	if blocked {
		items = append(items, workflow.EvidenceBlocker{ID: "QA1", Owner: "平台管理员", Reason: "缺少审批消费证据", ResolutionCondition: "提供原始消费时间"})
	}
	body, _ := json.Marshal(map[string]any{"iteration_complete": !blocked, "summary": "证据核验", "evidence_blockers": items})
	summary, _ := json.Marshal(map[string]any{"iteration_complete": !blocked, "summary": "证据核验"})
	id := uuid.NewString()
	return workflow.NodeOutput{Text: string(summary), Blockers: items, ReportIDs: []string{id}, ReportSHA256: workflow.ReportHash(string(body)), Reports: []workflow.Report{{ID: id, Title: "核验报告", Markdown: string(body)}}}
}

func TestHatchetEvidenceWaitSurvivesRestartAndOnlyRechecksBlockedNode(t *testing.T) {
	_, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	var mu sync.Mutex
	calls := map[string]int{}
	execute := func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[input.Node.ID]++
		if input.Node.ID == "qa" {
			return evidenceOutput(input.EvidenceRetry == nil), nil
		}
		return workflow.NodeOutput{Text: `{"iteration_complete":true,"summary":"独立通过"}`}, nil
	}
	stop := startReviewWorker(t, client, execute)
	t.Cleanup(func() {
		if stop != nil {
			_ = stop()
		}
	})
	run, err := backend.Submit(ctx, evidenceDefinition(project.ID), "evidence-wait")
	if err != nil {
		t.Fatal(err)
	}
	gate := awaitApproval(t, backend.Namespace(), run.RunID)
	if gate.Purpose != "evidence" {
		t.Fatal("未进入专用补证审核")
	}
	mu.Lock()
	count := calls["final"]
	mu.Unlock()
	if count != 0 {
		t.Fatal("阻塞后继续了下游")
	}
	if err = stop(); err != nil {
		t.Fatal(err)
	}
	stop = nil
	stop = startReviewWorker(t, client, execute)
	decision := approval.Decision{SubjectHash: gate.SubjectHash, Decision: "changes_requested", Reason: "审批记录已在平台索引中，请核对 consumed_at"}
	if _, err = approval.Decide(ctx, actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	finished := waitRun(t, api.New(backend, integrationAccess{}), run.RunID)
	mu.Lock()
	defer mu.Unlock()
	if finished.Status != "succeeded" || calls["plan"] != 1 || calls["qa"] != 2 || calls["final"] != 1 {
		t.Fatalf("重启或复验重跑了无关步骤: %s %v", finished.Status, calls)
	}
}
