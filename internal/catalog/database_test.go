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
	item, err := Create(context.Background(), namespace, NewObject{Kind: "agent", Name: "跨项目模板", Spec: Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Handoff: testHandoff(), Instruction: "检查项目"}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := Publish(context.Background(), namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return item, version
}

func TestSharedVersionRunsInTwoProjectsWithoutCopying(t *testing.T) {
	ns, first, second := catalogDatabase(t)
	ctx := context.Background()
	item, version := templateVersion(t, ns)
	for _, project := range []string{first, second} {
		d, err := Compile(ctx, ns, version.ID, project, "任务")
		if err != nil || d.ProjectID != project || d.Nodes[0].AgentVersion != version.ID {
			t.Fatal("共享版本执行上下文错误", err)
		}
	}
	item.Spec.Agent.Instruction = "新版指令"
	updated, err := Save(ctx, ns, item.ID, item.Revision, item.Name, item.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Publish(ctx, ns, item.ID, updated.Revision); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadVersion(ctx, ns, version.ID)
	if err != nil || stored.Spec.Agent.Instruction != "检查项目" {
		t.Fatal("旧版本被修改", err)
	}
	if _, err = Get(ctx, "foreign", item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("跨工作空间读取", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.catalog_versions SET spec='{}' WHERE id=$1", version.ID); err == nil {
		t.Fatal("发布版本可变")
	}
	if _, err = Compile(ctx, ns, version.ID, "", "任务"); err == nil {
		t.Fatal("未指定项目仍能运行")
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
	if _, err := Compile(ctx, ns, version.ID, project, "任务"); err == nil {
		t.Fatal("归档角色仍能启动新运行")
	}
	old, err := ReadVersion(ctx, ns, version.ID)
	if err != nil || old.Spec.Agent.Instruction != "检查项目" {
		t.Fatalf("归档破坏历史: %+v %v", old, err)
	}
}
