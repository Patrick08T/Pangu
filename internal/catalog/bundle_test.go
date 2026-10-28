package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func bundleFixture() Bundle {
	return Bundle{Name: "自动生成的流程", Agents: []AgentChange{{Key: "writer", Name: "生成助手", Node: &workflow.Node{ID: "writer", Executor: "codex", Instruction: "完成指定任务", ReadOnly: true, Handoff: testHandoff()}}}, Steps: []BundleStep{{ID: "first", Agent: "writer"}, {ID: "second", Agent: "writer", DependsOn: []string{"first"}}}}
}

func TestBundleRejectsCrossWorkspaceAndConcurrentChanges(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	ctx := context.Background()
	b := bundleFixture()
	item, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "已有助手", Spec: Spec{Agent: b.Agents[0].Node}})
	if err != nil {
		t.Fatal(err)
	}
	b.Agents[0].ObjectID, b.Agents[0].Revision = item.ID, 1
	if err = CheckBundle(ctx, "foreign", b); err == nil {
		t.Fatal("允许跨工作空间修改")
	}
	b.Agents[0].Revision = 2
	if err = CheckBundle(ctx, ns, b); !errors.Is(err, ErrConflict) {
		t.Fatal("允许错误修订", err)
	}
	stored, err := Get(ctx, ns, item.ID)
	if err != nil || stored.Revision != 1 {
		t.Fatal("校验修改了原对象", err)
	}
}

func TestBundleCreatesPublishedAgentsAndWorkflowAtomically(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	out, err := ApplyBundleTx(ctx, tx, ns, bundleFixture())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Agents) != 1 || out.Workflow == nil || out.Workflow.Spec.Flow.Nodes[0].AgentVersion != out.Agents[0].ID {
		t.Fatalf("引用未固定: %+v", out)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, out.Workflow.ID, project, "输入")
	if err != nil || len(d.Nodes) != 2 || d.Nodes[1].Handoff == nil {
		t.Fatalf("生成流程不可执行: %+v %v", d, err)
	}
}

func TestBundleValidationAndFailureLeaveNoPartialObjects(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	ctx := context.Background()
	bundle := bundleFixture()
	if err := CheckBundle(ctx, ns, bundle); err != nil {
		t.Fatal(err)
	}
	items, err := List(ctx, ns, "", 0)
	if err != nil || len(items) != 0 {
		t.Fatal("预览校验创建了资源", err)
	}
	bundle.Steps[0].DependsOn = []string{"second"}
	if err := CheckBundle(ctx, ns, bundle); err == nil {
		t.Fatal("循环依赖被接受")
	}
	items, _ = List(ctx, ns, "", 0)
	if len(items) != 0 {
		t.Fatal("失败留下部分对象")
	}
	bundle = bundleFixture()
	bundle.Agents[0].Node.Handoff = nil
	if err := CheckBundle(ctx, ns, bundle); err == nil {
		t.Fatal("缺少交接契约被接受")
	}
}
