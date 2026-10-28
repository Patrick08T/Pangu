package catalog

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
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
