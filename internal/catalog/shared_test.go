package catalog

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestSharedCatalogDefinitionsDoNotRequireProject(t *testing.T) {
	for _, item := range []NewObject{
		{Kind: "agent", Name: "开发工程师", Spec: Spec{Agent: &workflow.Node{ID: "developer", Executor: "codex", Instruction: "根据项目任务开发", Handoff: testHandoff()}}},
		{Kind: "workflow", Name: "性能优化", Spec: Spec{Flow: &Flow{Nodes: []Binding{}}}},
		{Kind: "connection", Name: "共享账号", Spec: Spec{Connection: &Connection{Provider: "codex"}}},
	} {
		t.Run(item.Kind, func(t *testing.T) {
			if err := item.Validate(); err != nil {
				t.Fatalf("共享配置不应要求项目: %v", err)
			}
		})
	}
}

func TestMergedRoleKeepsFixedAutomationVersionRunnable(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	kept, _ := templateVersion(t, ns)
	duplicate, version := templateVersion(t, ns)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.catalog_objects SET archived=true,merged_into=$1 WHERE id=$2", kept.ID, duplicate.ID); err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, version.ID, project, "已有计划的输入")
	if err != nil || d.VersionID != version.ID || d.Nodes[0].AgentVersion != version.ID {
		t.Fatal("合并破坏了自动化的固定版本", err)
	}
	items, err := List(ctx, ns, "agent", 0)
	if err != nil || len(items) != 1 || items[0].ID != kept.ID {
		t.Fatal("合并副本仍在目录中", err)
	}
	if err = Archive(ctx, ns, duplicate.ID, false); err == nil {
		t.Fatal("合并副本被独立恢复")
	}
	if err = Archive(ctx, ns, kept.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = Compile(ctx, ns, version.ID, project, "任务"); err == nil {
		t.Fatal("保留角色已归档，旧别名仍可运行")
	}
}

func TestPublishingSharedFlowDoesNotCreateAgents(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	ctx := context.Background()
	role, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "共享角色", Spec: Spec{Agent: &workflow.Node{ID: "reviewer", Executor: "codex", Instruction: "检查当前项目", Handoff: testHandoff()}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, role.ID, role.Revision)
	if err != nil {
		t.Fatal(err)
	}
	flow, err := Create(ctx, ns, NewObject{Kind: "workflow", Name: "共享流程", Spec: Spec{Flow: &Flow{Nodes: []Binding{{ID: "check", AgentVersion: v.ID}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Publish(ctx, ns, flow.ID, flow.Revision); err != nil {
		t.Fatal(err)
	}
	roles, err := List(ctx, ns, "agent", 0)
	if err != nil || len(roles) != 1 || roles[0].ID != role.ID {
		t.Fatalf("发布工作流不应复制角色: %+v %v", roles, err)
	}
}
