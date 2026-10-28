package workspace

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"testing"
)

func TestForkClonesSuccessfulCheckpointAndKeepsOriginalUntouched(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "fork-test")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := sourceRepository(t)
	project, err := Register(ctx, "fork-test", Project{Name: "返工", Source: source, Branch: "main", WorkerID: "fork-test"})
	if err != nil {
		t.Fatal(err)
	}
	old, node := uuid.NewString(), uuid.NewString()
	if err = Ensure(ctx, old, project.ID, "fork-test"); err != nil {
		t.Fatal(err)
	}
	_, err = Execute(ctx, old, node, 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "已完成"}, os.WriteFile(filepath.Join(dir, "result.txt"), []byte("原成功结果"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	commit, out, err := Checkpoint(ctx, old, node)
	if err != nil {
		t.Fatal(err)
	}
	next := uuid.NewString()
	if err = EnsureFork(ctx, next, project.ID, "fork-test", old, commit); err != nil {
		t.Fatal(err)
	}
	original, _ := Get(ctx, old)
	forked, _ := Get(ctx, next)
	if original.HeadCommit != forked.BaseCommit || forked.ID == old {
		t.Fatal("没有固定成功检查点")
	}
	values, err := ImportCheckpoints(ctx, next, &workflow.Fork{Checkpoints: map[string]string{"build": commit}, Reused: map[string]workflow.NodeOutput{"build": out}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = Checkpoint(ctx, next, values["build"].RunID); err != nil {
		t.Fatal("复用记录不能继续返工", err)
	}
	_, err = Execute(ctx, next, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		data, err := os.ReadFile(filepath.Join(dir, "result.txt"))
		if err != nil || string(data) != "原成功结果" {
			t.Fatal("未从检查点读取")
		}
		return workflow.NodeOutput{Text: "修改完成"}, os.WriteFile(filepath.Join(dir, "result.txt"), []byte("返工结果"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	unchanged, _ := Get(ctx, old)
	if unchanged.HeadCommit != original.HeadCommit {
		t.Fatal("覆盖旧历史")
	}
	if err = EnsureFork(ctx, uuid.NewString(), project.ID, "fork-test", old, "not-a-checkpoint"); err == nil {
		t.Fatal("接受任意 Git commit")
	}
}
