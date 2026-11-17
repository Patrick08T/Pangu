package approval

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/usage"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func planFixture(t *testing.T) (identity.Actor, string, string, workflow.Definition, workflow.NodeInput, *workflow.Fork) {
	t.Helper()
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	h := &workflow.HandoffContract{Objective: "目标", Inputs: "输入", Deliverables: "报告", Acceptance: "验证", FailureHandling: "暂停"}
	a, err := catalog.Create(ctx, actor.Namespace, catalog.NewObject{Kind: "agent", Name: "执行角色", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Instruction: "执行", Handoff: h}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, actor.Namespace, a.ID, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	flow := &catalog.Flow{Budget: &workflow.Budget{CodexSessions: 1}, Nodes: []catalog.Binding{{ID: "plan", AgentVersion: v.ID, ReadOnly: true}, {ID: "build", AgentVersion: v.ID, DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}, ExecutionPlan: true}}}}
	o, err := catalog.Create(ctx, actor.Namespace, catalog.NewObject{Kind: "workflow", Name: "计划续跑", Spec: catalog.Spec{Flow: flow}})
	if err != nil {
		t.Fatal(err)
	}
	fv, err := catalog.Publish(ctx, actor.Namespace, o.ID, o.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := catalog.Compile(ctx, actor.Namespace, fv.ID, project, "方案")
	if err != nil {
		t.Fatal(err)
	}
	body := withReview(`{"tasks":[{"id":"T1"}],"execution_plan":{"changes":[{"node_id":"build","task":"实施具体获批任务"}],"assignments":[{"task_id":"T1","node_id":"build"}]}}`)
	result := workflow.NodeOutput{Text: body, RunID: uuid.NewString()}
	input := d.NodeInput(d.Nodes[1], map[string]workflow.NodeOutput{"plan": result})
	fork := &workflow.Fork{OriginCommit: "base", Commit: "head", CheckpointNode: "plan", Checkpoints: map[string]string{"plan": "head"}, Reused: map[string]workflow.NodeOutput{"plan": result}}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	raw, _ := json.Marshal(submission.Snapshot{Workflow: *d, CreatedBy: actor.ID})
	if _, err = conn.Exec(ctx, "UPDATE pangu.submissions SET snapshot=$2 WHERE id=$1", request, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready)VALUES($1,$2,'base','head','test','unused',true)`, run, project); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(result)
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_checkpoints(node_run_id,run_id,commit_id,output)VALUES($1,$2,'head',$3)`, result.RunID, run, raw); err != nil {
		t.Fatal(err)
	}
	return actor, run, request, *d, input, fork
}

func TestPlanContinuationCannotResetCallBudget(t *testing.T) {
	actor, run, request, _, input, fork := planFixture(t)
	ctx := context.Background()
	if _, err := usage.Reserve(usage.WithRun(ctx, run, "plan", input.Parents["plan"].RunID, 1), "codex", "test", nil); err != nil {
		t.Fatal(err)
	}
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	items := append(acceptedItems(), workflow.ReviewAnswer{ItemID: "platform-execution-graph", Decision: "accept", SelectedOptionIDs: []string{"apply"}})
	if _, err = Decide(ctx, actor, gate.ID, Decision{Decision: "approved", SubjectHash: gate.SubjectHash, Items: items}); err != nil {
		t.Fatal(err)
	}
	next, err := Continue(ctx, actor.Namespace, gate.ID, fork)
	if err != nil {
		t.Fatal(err)
	}
	if err = execution.RequestCancel(ctx, actor.Namespace, run); err != nil {
		t.Fatal(err)
	}
	child := uuid.NewString()
	if _, err = submission.Dispatch(ctx, actor.Namespace, next.ID, func(context.Context, *submission.Record) (string, error) { return child, nil }); err != nil {
		t.Fatal(err)
	}
	if cancelled, e := execution.CancelRequested(ctx, child); e != nil || !cancelled {
		t.Fatal("发布后、派发前取消没有传给接续运行", e)
	}
	_, err = usage.Reserve(usage.WithRun(ctx, child, "build", uuid.NewString(), 1), "codex", "test", nil)
	if workflow.FailureKind(err) != "budget" {
		t.Fatalf("发布新版本重置了已有预算: %v", err)
	}
	report, err := usage.ForRun(ctx, child)
	if err != nil || report.Used.CodexSessions != 1 {
		t.Fatalf("未展示继承用量: %+v %v", report, err)
	}
	assertRetainedBudget(t, run, child)
}

func assertRetainedBudget(t *testing.T, parent, child string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.usage_calls SET created_at=now()-interval '91 days' WHERE run_id=$1", parent); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.submissions SET purged_at=now() WHERE run_id=$1", parent); err != nil {
		t.Fatal(err)
	}
	if err = usage.Maintain(ctx); err != nil {
		t.Fatal(err)
	}
	report, err := usage.ForRun(ctx, child)
	if err != nil || report.Used.CodexSessions != 1 {
		t.Fatalf("暂停接续丢失来源预算: %+v %v", report, err)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.submissions SET purged_at=now() WHERE run_id=$1", child); err != nil {
		t.Fatal(err)
	}
	if err = usage.Maintain(ctx); err != nil {
		t.Fatal(err)
	}
	report, err = usage.ForRun(ctx, child)
	if err != nil || report.Used.CodexSessions != 0 {
		t.Fatalf("全部过期后无法清理: %+v %v", report, err)
	}
}

func TestApprovedPlanPublishesOnceAndInheritsExactAuthorization(t *testing.T) {
	actor, run, request, d, input, fork := planFixture(t)
	ctx := context.Background()
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	if gate.Execution == nil || len(gate.Execution.Changes) != 1 || len(gate.Review.Items) != 2 {
		t.Fatal("审批未冻结执行图差异")
	}
	if _, err = Continue(ctx, actor.Namespace, gate.ID, fork); err == nil {
		t.Fatal("未审批就发布")
	}
	items := append(acceptedItems(), workflow.ReviewAnswer{ItemID: "platform-execution-graph", Decision: "accept", SelectedOptionIDs: []string{"apply"}})
	if _, err = Decide(ctx, actor, gate.ID, Decision{Decision: "approved", SubjectHash: gate.SubjectHash, Items: items}); err != nil {
		t.Fatal(err)
	}
	next, err := Continue(ctx, actor.Namespace, gate.ID, fork)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := Continue(ctx, actor.Namespace, gate.ID, fork)
	if err != nil || repeated.ID != next.ID {
		t.Fatal("重复发布续跑", err)
	}
	if next.Snapshot.Workflow.VersionID == d.VersionID || next.Snapshot.ApprovalOrigin != gate.ID {
		t.Fatal("未发布新版本或丢失授权来源")
	}
	old, err := catalog.Compile(ctx, actor.Namespace, d.VersionID, d.ProjectID, d.Input)
	if err != nil || old.Nodes[1].Instruction != d.Nodes[1].Instruction {
		t.Fatal("历史版本被修改", err)
	}
	child := uuid.NewString()
	if _, err = submission.Dispatch(ctx, actor.Namespace, next.ID, func(context.Context, *submission.Record) (string, error) { return child, nil }); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready)VALUES($1,$2,'head','head','test','child',true)`, child, d.ProjectID); err != nil {
		t.Fatal(err)
	}
	childInput := next.Snapshot.Workflow.NodeInput(next.Snapshot.Workflow.Nodes[1], fork.Reused)
	childGate, err := Request(ctx, actor.Namespace, next.ID, child, childInput)
	if err != nil {
		t.Fatal(err)
	}
	if childGate.InheritedFrom != gate.ID || childGate.Status != "approved" {
		t.Fatal("续跑再次要求人工审批或隐去了授权来源")
	}
	if err = Consume(ctx, child, childInput, childGate.ID); err != nil {
		t.Fatal(err)
	}
	childInput.Input = "改动后的需求"
	if err = Consume(ctx, child, childInput, childGate.ID); err == nil {
		t.Fatal("继承批准被用于其他需求")
	}
}
