package catalog

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func catalogDatabase(t *testing.T) (string, string, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	namespace := uuid.NewString()
	projects := []string{}
	for range 2 {
		p, err := workspace.Register(context.Background(), namespace, workspace.Project{Name: "模板测试", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
		if err != nil {
			t.Fatal(err)
		}
		projects = append(projects, p.ID)
	}
	return namespace, projects[0], projects[1]
}

func templateVersion(t *testing.T, namespace string) (*Object, *Version) {
	t.Helper()
	item, err := Create(context.Background(), namespace, NewObject{Kind: "template", Name: "跨项目模板", Spec: Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Instruction: "检查项目"}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := Publish(context.Background(), namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return item, version
}

func TestSharedTemplateBindingIsFixedAndPublishedVersionsAreImmutable(t *testing.T) {
	ns, first, second := catalogDatabase(t)
	ctx := context.Background()
	template, version := templateVersion(t, ns)
	a, err := Bind(ctx, ns, version.ID, first, "项目一", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Bind(ctx, ns, version.ID, second, "项目二", "")
	if err != nil || a.ID == b.ID || a.ProjectID != first || b.ProjectID != second {
		t.Fatalf("绑定错误: %+v %v", b, err)
	}
	template.Spec.Agent.Instruction = "新版指令"
	updated, err := Save(ctx, ns, template.ID, template.Revision, template.Name, template.Spec)
	if err != nil {
		t.Fatal(err)
	}
	newVersion, err := Publish(ctx, ns, template.ID, updated.Revision)
	if err != nil || newVersion.ID == version.ID {
		t.Fatalf("发布: %+v %v", newVersion, err)
	}
	stored, err := Get(ctx, ns, a.ID)
	if err != nil || stored.Spec.Agent.Instruction != "检查项目" || stored.Spec.TemplateVersion != version.ID {
		t.Fatalf("绑定被自动升级: %+v %v", stored, err)
	}
	if _, err := Get(ctx, "foreign", a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨命名空间读取: %v", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "UPDATE pangu.catalog_objects SET project_id=$1 WHERE id=$2", second, a.ID); err == nil {
		t.Fatal("数据库接受项目迁移")
	}
	if _, err := conn.Exec(ctx, "UPDATE pangu.catalog_versions SET spec='{}' WHERE id=$1", version.ID); err == nil {
		t.Fatal("发布版本可变")
	}
}

func TestCatalogRevisionConflictAndArchiveProtectHistory(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	item, version := templateVersion(t, ns)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Go(func() { _, err := Save(ctx, ns, item.ID, item.Revision, "修改", item.Spec); results <- err })
	}
	group.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("并发覆盖: %d %d", success, conflict)
	}
	if err := Archive(ctx, ns, item.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Bind(ctx, ns, version.ID, project, "拒绝绑定", ""); err == nil {
		t.Fatal("归档模板仍能新建绑定")
	}
	old, err := ReadVersion(ctx, ns, version.ID)
	if err != nil || old.Spec.Agent.Instruction != "检查项目" {
		t.Fatalf("归档破坏历史: %+v %v", old, err)
	}
}
