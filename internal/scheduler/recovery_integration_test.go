package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/artifact"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetCodexCrashPreservesCheckpointAndIsolatesPartialWrite(t *testing.T) {
	if runtime.GOOS != "windows" || os.Getenv("PANGU_TEST_BINARY") == "" || os.Getenv("CODEX_TEST_BINARY") == "" {
		t.Skip("需要 Windows、构建的 Worker 和原生进程测试夹具")
	}
	_, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	project := registerTestProject(t, backend)
	control := t.TempDir()
	t.Setenv("PANGU_CODEX_HOME", t.TempDir())
	t.Setenv("PANGU_CODEX_BINARY", os.Getenv("CODEX_TEST_BINARY"))
	first := startProcess(t)
	awaitWorker(t, backend)
	d := workflow.Definition{ProjectID: project.ID, Input: control, Nodes: []workflow.Node{
		{ID: "source", Executor: "codex", Model: "checkpoint-source", Instruction: "保留上游"},
		{ID: "recover", Executor: "codex", Model: "checkpoint-recover", Instruction: "隔离失败写入", DependsOn: []string{"source"}},
	}}
	accepted, err := backend.Submit(context.Background(), d, "crash-checkpoint")
	if err != nil || accepted.RunID == "" {
		t.Fatalf("提交: %+v %v", accepted, err)
	}
	awaitRecoveryMarker(t, filepath.Join(control, "crash-ready"))
	if err := first.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = first.Wait()
	startProcess(t)
	out := waitRun(t, api.New(backend, integrationAccess{}), accepted.RunID)
	if !out.StopConfirmed || len(out.Attempts) != 3 {
		t.Fatalf("停止凭据不完整: %+v", out.Attempts)
	}
	for model, count := range map[string]int{"checkpoint-source": 1, "checkpoint-recover": 2} {
		data, err := os.ReadFile(filepath.Join(control, model))
		if err != nil || strings.Count(string(data), "attempt") != count {
			t.Fatalf("%s: %s %v", model, data, err)
		}
	}
	manifest, err := artifact.List(context.Background(), accepted.RunID)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range manifest.Files {
		names[file.Path] = true
	}
	if !names["preserved.txt"] || !names["recovered.txt"] || names["poison.txt"] {
		t.Fatalf("检查点内容错误: %+v", manifest)
	}
}

func awaitRecoveryMarker(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("写入途中故障注入未就绪")
}
