package factory

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func TestOptimizationCannotRemoveApprovalOrItsDependencies(t *testing.T) {
	ctx, ns, _ := setup(t)
	var reply Reply
	parsed, err := Parse(testReply())
	if err != nil {
		t.Fatal(err)
	}
	reply = *parsed
	b := *reply.Proposal
	b.Steps = []catalog.BundleStep{{ID: "gate", Agent: "writer", Approval: &workflow.Approval{TimeoutSeconds: 60}}, {ID: "write", Agent: "writer", DependsOn: []string{"gate"}}}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	out, err := catalog.ApplyBundleTx(ctx, tx, ns, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	in := Input{Mode: "optimize", TargetID: out.Workflow.ObjectID, TargetRevision: out.Workflow.SourceRevision}
	b.ObjectID, b.Revision = in.TargetID, in.TargetRevision
	if err = checkOptimization(ctx, ns, in, &b); err != nil {
		t.Fatal(err)
	}
	b.Steps[1].DependsOn = nil
	if checkOptimization(ctx, ns, in, &b) == nil {
		t.Fatal("允许断开审批依赖")
	}
	b.Steps = b.Steps[1:]
	if checkOptimization(ctx, ns, in, &b) == nil {
		t.Fatal("允许删除审批节点")
	}
}

func TestOptimizationCannotWeakenContractOrIncreasePermissions(t *testing.T) {
	before := workflow.Node{Executor: "codex", ReadOnly: true, Policy: &workflow.Policy{}, Handoff: &workflow.HandoffContract{Acceptance: "所有检查通过"}}
	after := before
	after.Instruction = "优化职责描述"
	if err := nodeWithin(&before, &after); err != nil {
		t.Fatal(err)
	}
	after.ReadOnly = false
	if nodeWithin(&before, &after) == nil {
		t.Fatal("扩大文件权限")
	}
	after = before
	after.Policy = &workflow.Policy{Retries: 1}
	if nodeWithin(&before, &after) == nil {
		t.Fatal("扩大重试次数")
	}
	after = before
	after.Handoff = &workflow.HandoffContract{Acceptance: "任意结果"}
	if nodeWithin(&before, &after) == nil {
		t.Fatal("降低验收标准")
	}
}
