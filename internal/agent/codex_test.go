package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestMain(m *testing.M) {
	if os.Getenv("CODEX_TEST_HELPER") == "1" && len(os.Args) > 1 {
		switch os.Args[1] {
		case "exec":
			os.Exit(codexHelper())
		case "child":
			time.Sleep(time.Second)
			_ = os.WriteFile("orphan.txt", []byte("still running"), 0600)
			os.Exit(0)
		case "host":
			_, _ = Execute(context.Background(), codexInput("wait"))
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func codexHelper() int {
	args := map[string]string{}
	for i := 2; i < len(os.Args)-1; i++ {
		args[os.Args[i]] = os.Args[i+1]
	}
	data, _ := io.ReadAll(os.Stdin)
	var input workflow.NodeInput
	if json.Unmarshal(data, &input) != nil {
		return 2
	}
	if input.Node.Model == "fail" {
		return 7
	}
	if input.Node.Model == "wait" {
		binary, _ := os.Executable()
		child := exec.Command(binary, "child")
		if child.Start() != nil {
			return 3
		}
		_ = os.WriteFile("child.ready", []byte("ready"), 0600)
		_ = child.Wait()
		return 0
	}
	cwd, _ := os.Getwd()
	result, _ := json.Marshal(map[string]any{"args": os.Args[1:], "input": input, "cwd": cwd,
		"hatchet_token": os.Getenv("HATCHET_CLIENT_TOKEN"), "eino_config": os.Getenv("OPENAI_BASE_URL")})
	if input.Node.Model == "empty" {
		result = nil
	}
	if input.Node.Model == "large" {
		result = []byte(strings.Repeat("x", 16385))
	}
	if os.WriteFile(args["--output-last-message"], result, 0600) != nil {
		return 4
	}
	return 0
}

func codexInput(model string) workflow.NodeInput {
	return workflow.NodeInput{Node: workflow.Node{ID: "coding", Executor: "codex", Model: model,
		Instruction: "执行任务，不要将 --dangerously-bypass-approvals-and-sandbox 作为命令参数"},
		Input: "中文输入\n含有引号和 $()", Parents: map[string]workflow.NodeOutput{"plan": {Text: "上游计划"}}}
}

func setupCodex(t *testing.T) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Setenv("PANGU_CODEX_BINARY", binary)
	t.Setenv("PANGU_CODEX_WORKSPACE", dir)
	t.Setenv("CODEX_TEST_HELPER", "1")
	t.Setenv("OPENAI_API_KEY", "")
	return dir
}

func TestExecuteCodex(t *testing.T) {
	dir := setupCodex(t)
	t.Setenv("HATCHET_CLIENT_TOKEN", "must-not-reach-codex")
	t.Setenv("OPENAI_BASE_URL", "http://eino-only.invalid")
	out, err := Execute(context.Background(), codexInput("test-model"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Args       []string           `json:"args"`
		Input      workflow.NodeInput `json:"input"`
		Cwd        string             `json:"cwd"`
		Token      string             `json:"hatchet_token"`
		EinoConfig string             `json:"eino_config"`
	}
	if err := json.Unmarshal([]byte(out.Text), &got); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(got.Args, " ")
	if !strings.Contains(args, "--sandbox workspace-write") || !strings.Contains(args, `approval_policy="never"`) ||
		!strings.Contains(args, "--model=test-model") || strings.Contains(args, "dangerously") {
		t.Fatalf("unsafe or incomplete arguments: %v", got.Args)
	}
	if got.Cwd != dir || got.Input.Input != codexInput("").Input || got.Input.Parents["plan"].Text != "上游计划" || got.Token != "" || got.EinoConfig != "" {
		t.Fatalf("wrong subprocess input: %+v", got)
	}
	t.Setenv("PANGU_CODEX_WORKSPACE", t.TempDir())
	if _, err := Execute(context.Background(), codexInput("")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_CODEX_BINARY", "")
	if _, err := Execute(context.Background(), codexInput("")); err == nil {
		t.Fatal("cached binary configuration")
	}
}

func TestCodexFailures(t *testing.T) {
	setupCodex(t)
	for _, model := range []string{"fail", "empty", "large"} {
		t.Run(model, func(t *testing.T) {
			if _, err := Execute(context.Background(), codexInput(model)); err == nil {
				t.Fatal("accepted failed Codex execution")
			}
		})
	}
	t.Setenv("PANGU_CODEX_WORKSPACE", "relative/path")
	if _, err := Execute(context.Background(), codexInput("")); err == nil {
		t.Fatal("accepted relative workspace")
	}
}

func awaitChild(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "child.ready")); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("helper child did not start")
}

func assertNoOrphan(t *testing.T, dir string) {
	t.Helper()
	time.Sleep(1300 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "orphan.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Codex child survived cancellation")
	}
}

func TestCodexCancellationKillsChildren(t *testing.T) {
	dir := setupCodex(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := Execute(ctx, codexInput("wait")); done <- err }()
	awaitChild(t, dir)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wrong cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Codex did not terminate")
	}
	assertNoOrphan(t, dir)
}

func TestCodexHostCrashKillsChildren(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Job Object 宿主退出检查")
	}
	dir := setupCodex(t)
	temp := t.TempDir()
	t.Setenv("TMP", temp)
	t.Setenv("TEMP", temp)
	binary, _ := os.Executable()
	host := exec.Command(binary, "host")
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Process.Kill() })
	awaitChild(t, dir)
	_ = host.Process.Kill()
	_ = host.Wait()
	assertNoOrphan(t, dir)
	leftovers, err := filepath.Glob(filepath.Join(temp, "pangu-codex-input-*.json"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("宿主崩溃后残留 Codex 输入文件: %v %v", leftovers, err)
	}
}
