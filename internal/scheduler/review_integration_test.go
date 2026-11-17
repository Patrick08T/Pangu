package scheduler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
	"net/http"
	"sync"
	"testing"
	"time"
)

func reviewDefinition(project string) workflow.Definition {
	d := workflow.Definition{ProjectID: project, Input: "结构化审核集成验证"}
	ids := []string{}
	for i := 0; i < 16; i++ {
		id := fmt.Sprintf("plan%02d", i)
		n := workflow.Node{ID: id, Executor: "codex", ReadOnly: true, Instruction: "只读规划", Handoff: testHandoff()}
		if i > 0 {
			n.DependsOn = []string{ids[i-1]}
		}
		ids = append(ids, id)
		d.Nodes = append(d.Nodes, n)
	}
	d.Nodes = append(d.Nodes, workflow.Node{ID: "build", Executor: "codex", Instruction: "批准后实施", Handoff: testHandoff(), DependsOn: []string{"plan00", "plan15"}, Approval: &workflow.Approval{SourceNode: "plan15", RevisionNodes: ids}})
	d.Nodes[len(d.Nodes)-1].Bindings = map[string]workflow.FieldBinding{"title": {From: "plan15", Pointer: "/review/title", Type: "string"}}
	d.Nodes = append(d.Nodes, workflow.Node{ID: "check", Executor: "codex", ReadOnly: true, Instruction: "检查", Handoff: testHandoff(), DependsOn: []string{"build", "plan01"}})
	return d
}

func revisionReport(revised bool) string {
	proposal := "原建议"
	resolutions := []workflow.ReviewResolution{}
	if revised {
		proposal = "仅交付文档"
		resolutions = append(resolutions, workflow.ReviewResolution{ItemID: "scope", Status: "addressed", Explanation: "已收窄为文档"})
	}
	r := workflow.Review{Title: "范围审核", Summary: "人工决定后实施", Items: []workflow.ReviewItem{{ID: "scope", Category: "范围", Title: "文档范围", Proposal: proposal, Rationale: "最小交付", EvidenceRefs: []string{}, TaskIDs: []string{}}}, Resolutions: resolutions}
	body, _ := json.Marshal(map[string]any{"review": r})
	return string(body)
}

func awaitReviewRound(t *testing.T, backend scheduler.Backend, run string, round int) *approval.Gate {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		gates, err := approval.List(context.Background(), backend.Namespace(), run)
		if err != nil {
			t.Fatal(err)
		}
		for _, gate := range gates {
			if gate.Round == round {
				return &gate
			}
		}
		state, err := backend.GetRun(context.Background(), run)
		if err == nil && state.Status == "failed" {
			t.Fatalf("复审失败: %+v", state.Error)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("未等到审核轮次", round)
	return nil
}

func TestHatchetStructuredReviewRevisionAndRestart(t *testing.T) {
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
		if input.Node.ID == "build" || input.Node.ID == "check" {
			for id, p := range input.Parents {
				if id != "build" && p.Text != revisionReport(true) {
					return workflow.NodeOutput{}, fmt.Errorf("下游收到旧报告 %s", id)
				}
			}
			return workflow.NodeOutput{Text: "完成"}, nil
		}
		if f := input.ReviewFeedback; f != nil && (f.Items[0].Correction != "仅交付文档" || f.Round != 2) {
			return workflow.NodeOutput{}, fmt.Errorf("纠偏输入丢失")
		}
		return workflow.NodeOutput{Text: revisionReport(input.ReviewFeedback != nil)}, nil
	}
	stop := startReviewWorker(t, client, execute)
	t.Cleanup(func() {
		if stop != nil {
			_ = stop()
		}
	})
	submitted, err := backend.Submit(ctx, reviewDefinition(project.ID), "structured-review")
	if err != nil {
		t.Fatal(err)
	}
	first := awaitReviewRound(t, backend, submitted.RunID, 1)
	if _, err = approval.Decide(ctx, actor, first.ID, approval.Decision{SubjectHash: first.SubjectHash, Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "revise", Correction: "仅交付文档"}}}); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	second := awaitReviewRound(t, backend, submitted.RunID, 2)
	mu.Lock()
	before := calls["build"]
	mu.Unlock()
	if before != 0 {
		t.Fatal("复审前已实施")
	}
	if err = stop(); err != nil {
		t.Fatal(err)
	}
	stop = nil
	stop = startReviewWorker(t, client, execute)
	if _, err = approval.Decide(ctx, actor, second.ID, approval.Decision{SubjectHash: second.SubjectHash, Decision: "approved", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}}}); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	finished := waitRun(t, api.New(backend, integrationAccess{}), submitted.RunID)
	mu.Lock()
	defer mu.Unlock()
	assertReviewExecutions(t, finished, calls)
}

func assertReviewExecutions(t *testing.T, finished scheduler.RunDetails, calls map[string]int) {
	t.Helper()
	if len(finished.Nodes) != 34 || calls["build"] != 1 || calls["check"] != 1 {
		t.Fatalf("分页或恢复重复: nodes=%d calls=%v", len(finished.Nodes), calls)
	}
	for i := 0; i < 16; i++ {
		if calls[fmt.Sprintf("plan%02d", i)] != 2 {
			t.Fatal("重启重复规划", calls)
		}
	}
}

func startReviewWorker(t *testing.T, client *hatchet.Client, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error)) func() error {
	t.Helper()
	worker, err := scheduler.NewWorker(client, execute, func(context.Context, workflow.Definition) workflow.Preflight { return workflow.Preflight{Ready: true} })
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	awaitWorker(t, scheduler.NewBackend(client))
	return stop
}

func approvalWorkflow(project, model string) workflow.Definition {
	return workflow.Definition{ProjectID: project, Nodes: []workflow.Node{
		{ID: "plan", Executor: "codex", ReadOnly: true, Instruction: "规划", Handoff: testHandoff()},
		{ID: "build", Executor: "eino", Model: model, Instruction: "批准后实施", Handoff: testHandoff(), DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}},
	}}
}

func approvalIntegrationSetup(t *testing.T) (*modelServer, scheduler.Backend, http.Handler) {
	m, client := integrationClient(t)
	stop := startReviewWorker(t, client, func(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		if input.Node.ID == "plan" {
			if input.ReviewFeedback != nil {
				return workflow.NodeOutput{}, fmt.Errorf("模拟修订资源失败")
			}
			return workflow.NodeOutput{Text: revisionReport(false)}, nil
		}
		return agent.Execute(ctx, input)
	})
	t.Cleanup(func() { _ = stop() })
	backend := scheduler.NewBackend(client)
	return m, backend, api.New(backend, integrationAccess{})
}

func TestHatchetRevisionFailureNeverExecutesOldPlan(t *testing.T) {
	m, backend, _ := approvalIntegrationSetup(t)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	run, err := backend.Submit(ctx, approvalWorkflow(project.ID, "revision-failure"), "revision-failure")
	if err != nil {
		t.Fatal(err)
	}
	gate := awaitReviewRound(t, backend, run.RunID, 1)
	_, err = approval.Decide(ctx, actor, gate.ID, approval.Decision{SubjectHash: gate.SubjectHash, Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "revise", Correction: "调整范围"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	failed := waitFailedRun(t, backend, run.RunID)
	if failed.Error == nil || m.count("revision-failure") != 0 {
		t.Fatal("修订失败仍实施", failed.Error)
	}
	gates, err := approval.List(ctx, backend.Namespace(), run.RunID)
	if err != nil || len(gates) != 1 || gates[0].Status != "changes_requested" {
		t.Fatal("失败后伪造新审批或丢失纠偏", err)
	}
}
