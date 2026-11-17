package scheduler_test

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"strings"
	"sync"
	"testing"
)

func continuationDefinition(t *testing.T, namespace, project string) workflow.Definition {
	t.Helper()
	ctx := context.Background()
	a, err := catalog.Create(ctx, namespace, catalog.NewObject{Kind: "agent", Name: "续跑角色", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Instruction: "按获批方案执行", Handoff: testHandoff()}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, namespace, a.ID, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	flow := &catalog.Flow{Nodes: []catalog.Binding{
		{ID: "plan", AgentVersion: v.ID, ReadOnly: true},
		{ID: "build", AgentVersion: v.ID, DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}, ExecutionPlan: true}},
		{ID: "obsolete", AgentVersion: v.ID, ReadOnly: true, DependsOn: []string{"build"}},
		{ID: "final", AgentVersion: v.ID, ReadOnly: true, DependsOn: []string{"obsolete"}},
	}}
	o, err := catalog.Create(ctx, namespace, catalog.NewObject{Kind: "workflow", Name: "批准新图接续", Spec: catalog.Spec{Flow: flow}})
	if err != nil {
		t.Fatal(err)
	}
	fv, err := catalog.Publish(ctx, namespace, o.ID, o.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := catalog.Compile(ctx, namespace, fv.ID, project, "接续测试")
	if err != nil {
		t.Fatal(err)
	}
	return *d
}

func continuationReport() string {
	var report map[string]any
	_ = json.Unmarshal([]byte(revisionReport(false)), &report)
	report["tasks"] = []map[string]string{{"id": "T1"}, {"id": "T2"}}
	report["execution_plan"] = map[string]any{"changes": []map[string]any{{"node_id": "build", "task": "实施已批准增量"}, {"node_id": "obsolete", "remove": true}, {"node_id": "final", "depends_on": []string{"build"}}}, "assignments": []workflow.TaskAssignment{{TaskID: "T1", NodeID: "build"}, {TaskID: "T2", NodeID: "final"}}}
	raw, _ := json.Marshal(report)
	return string(raw)
}

func TestHatchetApprovedGraphPublishesAndContinuesWithoutReplanning(t *testing.T) {
	_, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	d := continuationDefinition(t, backend.Namespace(), project.ID)
	var mu sync.Mutex
	calls := map[string]int{}
	buildRun := ""
	stop := startReviewWorker(t, client, func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[input.Node.ID]++
		if input.Node.ID == "plan" {
			return workflow.NodeOutput{Text: continuationReport()}, nil
		}
		if input.Node.ID == "build" {
			buildRun = input.RunID
			if !strings.Contains(input.Node.Instruction, "实施已批准增量") {
				t.Error("执行了旧任务")
			}
		}
		return workflow.NodeOutput{Text: `{"iteration_complete":true,"ready":true,"status":"complete","summary":"真实接续验证"}`}, nil
	})
	t.Cleanup(func() { _ = stop() })
	run, err := backend.Submit(ctx, d, "approved-graph")
	if err != nil {
		t.Fatal(err)
	}
	gate := awaitApproval(t, backend.Namespace(), run.RunID)
	items := []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}, {ItemID: "platform-execution-graph", Decision: "accept", SelectedOptionIDs: []string{"apply"}}}
	if _, err = approval.Decide(ctx, actor, gate.ID, approval.Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: items}); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	h := api.New(backend, integrationAccess{})
	original := waitRun(t, h, run.RunID)
	if original.Successor == nil || original.Successor.RunID == "" || original.Outcome.State != "continued" {
		t.Fatalf("缺少接续关联: %+v", original.Successor)
	}
	finished := waitRun(t, h, original.Successor.RunID)
	if finished.Definition.VersionID == d.VersionID || finished.SourceRunID != run.RunID || finished.Outcome.State != "accepted" {
		t.Fatalf("接续版本或验收错误: %+v", finished)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls["plan"] != 1 || calls["build"] != 1 || calls["final"] != 1 || calls["obsolete"] != 0 || buildRun != finished.ID {
		t.Fatalf("重复规划或执行旧图: %v build=%s", calls, buildRun)
	}
}
