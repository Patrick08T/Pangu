package scheduler_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func registerTestProject(t *testing.T, backend scheduler.Backend) *workspace.Project {
	t.Helper()
	root, source := t.TempDir(), t.TempDir()
	t.Setenv("PANGU_WORKSPACE_ROOT", root)
	t.Setenv("PANGU_WORKER_ID", "integration-worker")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("# 验证项目\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "--initial-branch=main"}, {"add", "."}, {"-c", "user.name=Pangu", "-c", "user.email=pangu@localhost", "commit", "-m", "基准"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("准备项目: %v %s", err, out)
		}
	}
	project, err := backend.RegisterProject(context.Background(), workspace.Project{Name: "真实调度验证项目", Source: source, Branch: "main", WorkerID: "integration-worker"})
	if err != nil {
		t.Fatal(err)
	}
	return project
}
