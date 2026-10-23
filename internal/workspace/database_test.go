package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestRegisteredProjectPersistsIsolatedRunAndCheckpoint(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "test-worker")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := sourceRepository(t)
	project, err := Register(ctx, "test", Project{Name: "演练", Source: source, Branch: "main", WorkerID: "test-worker"})
	if err != nil {
		t.Fatal(err)
	}
	run, node := uuid.NewString(), uuid.NewString()
	if err := Ensure(ctx, run, project.ID, "test"); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(ctx, run, project.ID, "other"); err == nil {
		t.Fatal("接受其他命名空间")
	}
	_, err = Execute(ctx, run, node, 1, false, func(dir string) (workflow.NodeOutput, error) {
		_ = os.WriteFile(filepath.Join(dir, "failed.txt"), []byte("失败"), 0600)
		return workflow.NodeOutput{}, errors.New("故意失败")
	})
	if err == nil {
		t.Fatal("失败尝试被发布")
	}
	out, err := Execute(ctx, run, node, 2, false, func(dir string) (workflow.NodeOutput, error) {
		if _, err := os.Stat(filepath.Join(dir, "failed.txt")); !os.IsNotExist(err) {
			t.Fatal("失败文件污染重试")
		}
		_ = os.WriteFile(filepath.Join(dir, "success.txt"), []byte("成功"), 0600)
		return workflow.NodeOutput{Text: "真实成功"}, nil
	})
	if err != nil || out.Text != "真实成功" {
		t.Fatalf("执行失败: %+v %v", out, err)
	}
	_, err = Execute(ctx, run, node, 3, false, func(string) (workflow.NodeOutput, error) {
		t.Fatal("已有检查点却重复执行")
		return workflow.NodeOutput{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_WORKER_ID", "other-worker")
	if err := Ensure(ctx, run, project.ID, "test"); err == nil {
		t.Fatal("其他 Worker 接管了本地磁盘")
	}
}

func TestCancelIntentPreventsCheckpointPublication(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "cancel-worker")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := sourceRepository(t)
	project, err := Register(ctx, "cancel", Project{Name: "取消竞争", Source: source, Branch: "main", WorkerID: "cancel-worker"})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if err := Ensure(ctx, run, project.ID, "cancel"); err != nil {
		t.Fatal(err)
	}
	before, _ := Get(ctx, run)
	_, err = Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		_ = os.WriteFile(filepath.Join(dir, "cancelled.txt"), []byte("不得发布"), 0600)
		conn, err := database.Open(ctx)
		if err != nil {
			return workflow.NodeOutput{}, err
		}
		defer conn.Close(context.Background())
		if _, err := conn.Exec(ctx, "INSERT INTO pangu.run_cancellations(run_id,namespace) VALUES($1,$2)", run, "cancel"); err != nil {
			t.Fatal(err)
		}
		return workflow.NodeOutput{Text: "结果已生成但取消已先持久化"}, nil
	})
	after, readErr := Get(ctx, run)
	if err == nil || readErr != nil || after.HeadCommit != before.HeadCommit {
		t.Fatalf("取消后仍发布文件: %v %v", err, readErr)
	}
}

func TestRerunUsesOriginalBaseInsteadOfSourceBranchOrPreviousResult(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "rerun-worker")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source, base := sourceRepository(t)
	project, err := Register(ctx, "rerun", Project{Name: "重跑", Source: source, Branch: "main", WorkerID: "rerun-worker"})
	if err != nil {
		t.Fatal(err)
	}
	first, next := uuid.NewString(), uuid.NewString()
	if err := Ensure(ctx, first, project.ID, "rerun"); err != nil {
		t.Fatal(err)
	}
	_, err = Execute(ctx, first, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "完成"}, os.WriteFile(filepath.Join(dir, "result.txt"), []byte("上次结果"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(source, "new.txt"), []byte("源分支推进"), 0600)
	if _, err := git(ctx, source, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, source, "commit", "-m", "后续修改"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRerun(ctx, next, project.ID, "rerun", first); err != nil {
		t.Fatal(err)
	}
	record, err := Get(ctx, next)
	if err != nil || record.BaseCommit != base || record.HeadCommit != base {
		t.Fatalf("重跑未固定原基准: %+v %v", record, err)
	}
	for _, name := range []string{"new.txt", "result.txt"} {
		if _, err := os.Stat(filepath.Join(record.Directory, "base", name)); !os.IsNotExist(err) {
			t.Fatalf("重跑继承了 %s", name)
		}
	}
}
