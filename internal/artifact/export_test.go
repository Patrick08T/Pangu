package artifact

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func artifactRun(t *testing.T) string {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "artifact-test")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "old.txt"), []byte("remove me\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "--initial-branch=main"}, {"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@localhost", "-c", "commit.gpgSign=false", "commit", "-m", "基准"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Git测试夹具: %s %v", data, err)
		}
	}
	project, err := workspace.Register(ctx, "artifact", workspace.Project{Name: "产物", Source: source, Branch: "main", WorkerID: "artifact-test"})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if err := workspace.Ensure(ctx, run, project.ID, "artifact"); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestArtifactRoundTripPreservesBinaryAndDeletion(t *testing.T) {
	run := artifactRun(t)
	ctx := context.Background()
	_, err := workspace.Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		if err := os.WriteFile(filepath.Join(dir, "binary.bin"), []byte{0, 255, 32, 10}, 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		if err := os.WriteFile(filepath.Join(dir, "中文.txt"), []byte("内容\n"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{Text: "完成"}, os.Remove(filepath.Join(dir, "old.txt"))
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := List(ctx, run)
	if err != nil || len(manifest.Files) != 3 {
		t.Fatalf("清单错误: %+v %v", manifest, err)
	}
	for _, file := range manifest.Files {
		if file.Path == "binary.bin" {
			_, data, _, err := Read(ctx, run, file.ID, false)
			if err != nil || !file.Binary || !bytes.Equal(data, []byte{0, 255, 32, 10}) {
				t.Fatalf("二进制损坏: %x %v", data, err)
			}
		}
		if file.Change == "D" {
			_, diff, _, err := Read(ctx, run, file.ID, true)
			if err != nil || !bytes.Contains(diff, []byte("-remove me")) {
				t.Fatalf("删除差异缺失: %q %v", diff, err)
			}
		}
	}
	data, err := Export(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) != 3 {
		t.Fatalf("导出清单错误: %v", err)
	}
	if _, _, _, err := Read(ctx, run, "../old.txt", false); !errors.Is(err, ErrNotFound) {
		t.Fatal("接受任意路径")
	}
}

func TestArtifactBlocksSecretsAndTruncatesLargePreview(t *testing.T) {
	run := artifactRun(t)
	ctx := context.Background()
	_, err := workspace.Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		if err := os.WriteFile(filepath.Join(dir, "large.txt"), []byte(strings.Repeat("内容\n", 200000)), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{Text: "完成"}, os.WriteFile(filepath.Join(dir, "secret.txt"), []byte(`{"password":"hidden-value"}`), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := List(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range manifest.Files {
		if file.Path == "secret.txt" && file.Blocked != "sensitive" {
			t.Fatal("凭据未阻断")
		}
		if file.Path == "large.txt" {
			_, data, truncated, err := Read(ctx, run, file.ID, true)
			if err != nil || !truncated || len(data) > 1024*1024 {
				t.Fatalf("预览没有截断: %d %t %v", len(data), truncated, err)
			}
		}
	}
	if _, err := Export(ctx, run); !errors.Is(err, ErrBlocked) {
		t.Fatalf("含凭据仍可导出: %v", err)
	}
}

func TestDiffUsesLiteralPathAndCannotSelectSecretNeighbor(t *testing.T) {
	run := artifactRun(t)
	ctx := context.Background()
	_, err := workspace.Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		if err := os.WriteFile(filepath.Join(dir, "report[1].txt"), []byte("public report\n"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{Text: "完成"}, os.WriteFile(filepath.Join(dir, "report1.txt"), []byte("-----BEGIN PRIVATE KEY-----\nprivate-body\n-----END PRIVATE KEY-----"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := List(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range manifest.Files {
		if file.Path != "report[1].txt" {
			continue
		}
		_, data, _, err := Read(ctx, run, file.ID, true)
		if err != nil || !bytes.Contains(data, []byte("public report")) || bytes.Contains(data, []byte("private-body")) {
			t.Fatalf("差异越界: %v", err)
		}
		return
	}
	t.Fatal("缺少带方括号的文件")
}
