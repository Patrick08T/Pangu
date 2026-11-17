package catalog

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestRunnableChoicesOnlyReturnLatestPublishedSharedVersions(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	ctx := context.Background()
	_, version := templateVersion(t, ns)
	items, err := RunnableChoices(ctx, ns, 0)
	if err != nil || len(items) != 1 || items[0].Version.ID != version.ID {
		t.Fatal("共享已发布版本不可选", err)
	}
	first := items[0]
	spec := Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Handoff: testHandoff(), Instruction: "新版检查"}}
	updated, err := Save(ctx, ns, first.Version.ObjectID, first.Version.SourceRevision, "新版助手", spec)
	if err != nil {
		t.Fatal(err)
	}
	items, err = RunnableChoices(ctx, ns, 0)
	if err != nil || len(items) != 1 || items[0].Version.ID != first.Version.ID {
		t.Fatal("草稿替代了发布版本", err)
	}
	v, err := Publish(ctx, ns, updated.ID, updated.Revision)
	if err != nil {
		t.Fatal(err)
	}
	items, err = RunnableChoices(ctx, ns, 0)
	if err != nil || len(items) != 1 || items[0].Version.ID != v.ID || items[0].Name != "新版助手" {
		t.Fatal("未读取最新发布版本", err)
	}
	if _, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "仅草稿", Spec: spec}); err != nil {
		t.Fatal(err)
	}
	if err := Archive(ctx, ns, updated.ID, true); err != nil {
		t.Fatal(err)
	}
	items, err = RunnableChoices(ctx, ns, 0)
	if err != nil || len(items) != 0 {
		t.Fatal("列出了归档或未发布配置", err)
	}
}

func TestAgentChoicesExcludeUnavailableAndNonAgentObjects(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	ctx := context.Background()
	kept, version := templateVersion(t, ns)
	archived, _ := templateVersion(t, ns)
	if err := Archive(ctx, ns, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	templateVersion(t, uuid.NewString())
	if _, err := Create(ctx, ns, kept.NewObject); err != nil {
		t.Fatal(err)
	}
	for _, input := range []NewObject{
		{Kind: "connection", Name: "模型连接", Spec: Spec{Connection: &Connection{Provider: "codex"}}},
		{Kind: "workflow", Name: "工作流", Spec: Spec{Flow: &Flow{Nodes: []Binding{{ID: "check", AgentVersion: version.ID}}}}},
	} {
		item, err := Create(ctx, ns, input)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Publish(ctx, ns, item.ID, item.Revision); err != nil {
			t.Fatal(err)
		}
	}
	merged, _ := templateVersion(t, ns)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.catalog_objects SET archived=true,merged_into=$1 WHERE id=$2", kept.ID, merged.ID); err != nil {
		t.Fatal(err)
	}
	items, err := AgentChoices(ctx, ns)
	if err != nil || len(items) != 1 || items[0].Version.ID != version.ID {
		t.Fatalf("角色候选未隔离未发布、归档、合并、跨工作空间或非角色对象: %d %v", len(items), err)
	}
	empty, err := AgentChoices(ctx, uuid.NewString())
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("空角色仓库应返回完整空清单", err)
	}
}

func TestAgentChoicesDoNotTruncateAtListPageBoundary(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	want := map[string]bool{}
	for range 101 {
		_, v := templateVersion(t, ns)
		want[v.ID] = true
	}
	items, err := AgentChoices(context.Background(), ns)
	if err != nil || len(items) != len(want) {
		t.Fatalf("角色仓库在列表分页处截断: %d/101 %v", len(items), err)
	}
	for _, item := range items {
		if !want[item.Version.ID] {
			t.Fatal("重复或错误角色版本", item.Version.ID)
		}
		delete(want, item.Version.ID)
	}
}
