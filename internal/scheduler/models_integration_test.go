package scheduler_test

import (
	"context"
	"os"
	"testing"
)

func TestHatchetCodexModelChoicesReadWorkerSource(t *testing.T) {
	binary := os.Getenv("PANGU_TEST_CODEX_BINARY")
	if binary == "" {
		t.Skip("需要本机 Codex 程序；只读取内置目录，不调用模型")
	}
	t.Setenv("PANGU_CODEX_BINARY", binary)
	t.Setenv("PANGU_CODEX_HOME", t.TempDir())
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	_, backend, _ := integrationSetup(t)
	items, err := backend.CodexModels(context.Background())
	if err != nil || len(items) == 0 {
		t.Fatalf("Worker 模型目录不可用：%v %v", items, err)
	}
	t.Setenv("PANGU_CODEX_BINARY", "")
	if _, err = backend.CodexModels(context.Background()); err == nil {
		t.Fatal("模型目录被缓存，未重新读取 Worker 原始配置")
	}
}
