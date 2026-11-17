package scheduler_test

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evidence"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"strings"
	"sync"
	"testing"
)

func repairDefinition(t *testing.T, namespace, project string) (workflow.Definition, *catalog.Object) {
	t.Helper()
	ctx := context.Background()
	agent, err := catalog.Create(ctx, namespace, catalog.NewObject{Kind: "agent", Name: "补证修正角色", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Instruction: "核验", Handoff: testHandoff()}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, namespace, agent.ID, agent.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d := evidenceDefinition(project)
	flow := &catalog.Flow{Nodes: []catalog.Binding{}}
	for _, n := range d.Nodes {
		flow.Nodes = append(flow.Nodes, catalog.Binding{ID: n.ID, AgentVersion: v.ID, ReadOnly: true, DependsOn: n.DependsOn, OutputSchema: n.OutputSchema, ReportSchema: n.ReportSchema})
	}
	o, err := catalog.Create(ctx, namespace, catalog.NewObject{Kind: "workflow", Name: "证据修正", Spec: catalog.Spec{Flow: flow}})
	if err != nil {
		t.Fatal(err)
	}
	fv, err := catalog.Publish(ctx, namespace, o.ID, o.Revision)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := catalog.Compile(ctx, namespace, fv.ID, project, d.Input)
	if err != nil {
		t.Fatal(err)
	}
	flow.Nodes[1].Task = "执行修正核验"
	candidate, err := catalog.Create(ctx, namespace, catalog.NewObject{Kind: "workflow", Name: "修正候选", Spec: catalog.Spec{Flow: flow}})
	if err != nil {
		t.Fatal(err)
	}
	return *compiled, candidate
}

func TestHatchetEvidenceRepairPublishesOnceAndKeepsCompletedPrefix(t *testing.T) {
	_, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	d, candidate := repairDefinition(t, backend.Namespace(), project.ID)
	var mu sync.Mutex
	calls := map[string]int{}
	execute := func(_ context.Context, in workflow.NodeInput) (workflow.NodeOutput, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[in.Node.ID]++
		if in.Node.ID == "qa" {
			return evidenceOutput(!strings.Contains(in.Node.Instruction, "执行修正核验")), nil
		}
		return workflow.NodeOutput{Text: `{"iteration_complete":true,"ready":true,"status":"complete","summary":"独立通过"}`}, nil
	}
	stop := startReviewWorker(t, client, execute)
	t.Cleanup(func() {
		if stop != nil {
			_ = stop()
		}
	})
	run, err := backend.Submit(ctx, d, "evidence-repair")
	if err != nil {
		t.Fatal(err)
	}
	gate := awaitApproval(t, backend.Namespace(), run.RunID)
	p, err := backend.PrepareRepair(ctx, gate.ID, approval.RepairInput{SubjectHash: gate.SubjectHash, ObjectID: candidate.ID, Revision: candidate.Revision, CheckpointNode: "plan", Summary: "保留计划，修正审核后独立验收"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ReusedNodes) != 1 || len(p.ExecuteNodes) != 2 {
		t.Fatalf("复用范围错误: %+v", p)
	}
	p = repairConflictChecks(t, ctx, backend, actor, gate, candidate, p)
	decision := approval.Decision{SubjectHash: gate.SubjectHash, RepairHash: strings.Repeat("0", 64), Decision: "approved"}
	if _, err = approval.Decide(ctx, actor, gate.ID, decision); !errors.Is(err, approval.ErrConflict) {
		t.Fatalf("错误摘要可批准: %v", err)
	}
	if err = stop(); err != nil {
		t.Fatal(err)
	}
	stop = nil
	decision.RepairHash = p.Hash
	if _, err = approval.Decide(ctx, actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
	if _, err = approval.Decide(ctx, actor, gate.ID, decision); err != nil {
		t.Fatal("重复批准", err)
	}
	stop = startReviewWorker(t, client, execute)
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	original := waitRun(t, api.New(backend, integrationAccess{}), run.RunID)
	if original.Successor == nil || original.Successor.RunID == "" {
		t.Fatalf("没有修正续跑: %+v", original.Error)
	}
	finished := waitRun(t, api.New(backend, integrationAccess{}), original.Successor.RunID)
	if finished.Outcome.State != "accepted" || finished.Definition.VersionID == d.VersionID {
		t.Fatalf("新版本验收错误: %+v", finished.Outcome)
	}
	checkRepairRepeat(t, ctx, backend.Namespace(), gate.ID, original.Successor.ID)
	checkRepairEvidence(t, ctx, finished.ID, project.ID, p.Hash)
	mu.Lock()
	defer mu.Unlock()
	checkRepairCalls(t, calls)
}

func repairConflictChecks(t *testing.T, ctx context.Context, backend scheduler.Backend, actor identity.Actor, gate *approval.Gate, candidate *catalog.Object, p *approval.Repair) *approval.Repair {
	t.Helper()
	decision := approval.Decision{SubjectHash: gate.SubjectHash, RepairHash: p.Hash, Decision: "approved"}
	tokenActor := actor
	tokenActor.TokenID = "not-human"
	if _, err := approval.Decide(ctx, tokenActor, gate.ID, decision); !errors.Is(err, identity.ErrForbidden) {
		t.Fatal("Token 代替了人工批准", err)
	}
	candidate.Spec.Flow.Nodes[1].Task = "执行修正核验，新修订"
	changed, err := catalog.Save(ctx, backend.Namespace(), candidate.ID, candidate.Revision, candidate.Name, candidate.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = approval.Decide(ctx, actor, gate.ID, decision); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal("旧方案批准了变化后的草稿", err)
	}
	next, err := backend.PrepareRepair(ctx, gate.ID, approval.RepairInput{SubjectHash: gate.SubjectHash, ObjectID: changed.ID, Revision: changed.Revision, CheckpointNode: "plan", Summary: "保留计划，新修订核验后独立验收"})
	if err != nil || next.Hash == p.Hash {
		t.Fatal("新修订未形成独立摘要", err)
	}
	if _, err = approval.Decide(ctx, actor, gate.ID, decision); !errors.Is(err, approval.ErrConflict) {
		t.Fatal("过期方案仍可批准", err)
	}
	return next
}

func checkRepairEvidence(t *testing.T, ctx context.Context, run, project, hash string) {
	t.Helper()
	bundle, err := evidence.Load(ctx, run, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range bundle.Files {
		if file.Kind == "repairs" && strings.Contains(file.Body, hash) && strings.Contains(file.Body, "consumed") {
			return
		}
	}
	t.Fatal("新执行节点无法核对修正批准及生效版本")
}

func checkRepairRepeat(t *testing.T, ctx context.Context, namespace, gate, request string) {
	t.Helper()
	repeated, err := approval.ContinueRepair(ctx, namespace, gate)
	if err != nil || repeated.ID != request {
		t.Fatal("重复消费创建多个运行", err)
	}
}

func checkRepairCalls(t *testing.T, calls map[string]int) {
	t.Helper()
	if calls["plan"] != 1 || calls["qa"] != 2 || calls["final"] != 1 {
		t.Fatalf("重复规划或旧图继续: %v", calls)
	}
}
