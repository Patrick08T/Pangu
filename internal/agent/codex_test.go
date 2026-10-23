package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	if len(os.Args) > 2 && os.Args[1] == "verify-helper" {
		switch os.Args[2] {
		case "pass":
			fmt.Print("验证通过")
			os.Exit(0)
		case "fail":
			fmt.Print("验证失败")
			os.Exit(7)
		case "wait":
			time.Sleep(time.Minute)
			os.Exit(0)
		}
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "linger":
			time.Sleep(30 * time.Second)
			os.Exit(0)
		case "stdio":
			fmt.Fprint(os.Stdout, "native-output")
			fmt.Fprint(os.Stderr, "native-error")
			os.Exit(0)
		case "login":
			os.Exit(0)
		case "exec":
			if os.Getenv("PANGU_CODEX_POLICY_PROBE") == "1" {
				if _, err := os.Stat(filepath.Join(os.Getenv("CODEX_HOME"), "unsupported")); err == nil {
					fmt.Print("stock-codex")
				} else {
					fmt.Print("pangu-codex-policy/1")
				}
				os.Exit(0)
			}
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

func TestCodexRejectsRuntimeWithoutHardLimits(t *testing.T) {
	setupCodex(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("PANGU_CODEX_HOME"), "unsupported"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Execute(context.Background(), codexInput("model"))
	if err == nil || workflow.CanRetry(err) || workflow.FailureKind(err) != "parameter" {
		t.Fatal("未拒绝缺少硬限制的运行时", err)
	}
}

func TestProcessHonorsOutputWriters(t *testing.T) {
	dir := setupCodex(t)
	var output, diagnostic bytes.Buffer
	cmd := exec.CommandContext(context.Background(), os.Getenv("PANGU_CODEX_BINARY"), "stdio")
	cmd.Dir, cmd.Env = dir, codexEnvironment()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader(""), &output, &diagnostic
	if err := runCodexProcess(context.Background(), cmd, ""); err != nil {
		t.Fatal(err)
	}
	if output.String() != "native-output" || diagnostic.String() != "native-error" {
		t.Fatalf("输出未交给受控收集器: %q %q", output.String(), diagnostic.String())
	}
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
	fmt.Fprintln(os.Stdout, `{"type":"item.completed","item":{"type":"command_execution","id":"test","command":"go test ./...","aggregated_output":"ok","exit_code":0}}`)
	if input.Node.Model == "fail" {
		return 7
	}
	if strings.HasPrefix(input.Node.Model, "checkpoint-") {
		return checkpointHelper(input.Node.Model, args["--output-last-message"], input.Input)
	}
	if input.Node.Model == "wait" {
		fmt.Fprint(os.Stderr, "等待子进程取消")
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
		"hatchet_token": os.Getenv("HATCHET_CLIENT_TOKEN"), "eino_config": os.Getenv("OPENAI_BASE_URL"),
		"personal_secret": os.Getenv("PERSONAL_SERVICE_TOKEN"), "codex_home": os.Getenv("CODEX_HOME"), "user_home": os.Getenv("USERPROFILE")})
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

func checkpointHelper(model, output, control string) int {
	if !filepath.IsAbs(control) {
		return 9
	}
	file, err := os.OpenFile(filepath.Join(control, model), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 10
	}
	_, err = file.WriteString("attempt\n")
	_ = file.Close()
	if err != nil {
		return 11
	}
	if model == "checkpoint-source" {
		if os.WriteFile("preserved.txt", []byte("上游检查点"), 0600) != nil {
			return 12
		}
	} else {
		if data, err := os.ReadFile("preserved.txt"); err != nil || string(data) != "上游检查点" {
			return 13
		}
		marker := filepath.Join(control, "crash-ready")
		if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
			if os.WriteFile("poison.txt", []byte("失败写入"), 0600) != nil {
				return 14
			}
			if os.WriteFile(marker, []byte("ready"), 0600) != nil {
				return 15
			}
			time.Sleep(5 * time.Minute)
			return 16
		}
		if _, err := os.Stat("poison.txt"); !errors.Is(err, os.ErrNotExist) {
			return 17
		}
		if os.WriteFile("recovered.txt", []byte("恢复成功"), 0600) != nil {
			return 18
		}
	}
	if os.WriteFile(output, []byte("检查点验证通过"), 0600) != nil {
		return 19
	}
	return 0
}

func TestFailedNativeOutputRetainsCommandEvidence(t *testing.T) {
	setupCodex(t)
	for _, model := range []string{"empty", "large", "fail"} {
		out, err := Execute(context.Background(), codexInput(model))
		if err == nil || len(out.Evidence) != 1 || out.Evidence[0].ExitCode != 0 {
			t.Fatalf("%s 的错误丢失原生命令证据: %+v %v", model, out, err)
		}
	}
}

func codexInput(model string) workflow.NodeInput {
	return workflow.NodeInput{Workspace: os.Getenv("CODEX_TEST_WORKSPACE"), Node: workflow.Node{ID: "coding", Executor: "codex", Model: model,
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
	t.Setenv("PANGU_WORKSPACE_ROOT", dir)
	t.Setenv("PANGU_CODEX_HOME", t.TempDir())
	t.Setenv("CODEX_TEST_WORKSPACE", dir)
	t.Setenv("CODEX_TEST_HELPER", "1")
	t.Setenv("OPENAI_API_KEY", "")
	return dir
}

func TestCodexUsesDedicatedEnvironment(t *testing.T) {
	setupCodex(t)
	t.Setenv("PERSONAL_SERVICE_TOKEN", "must-not-inherit")
	t.Setenv("CODEX_HOME", t.TempDir())
	out, err := Execute(context.Background(), codexInput(""))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.Text), &got); err != nil {
		t.Fatal(err)
	}
	var secret, home, userHome string
	_ = json.Unmarshal(got["personal_secret"], &secret)
	_ = json.Unmarshal(got["codex_home"], &home)
	_ = json.Unmarshal(got["user_home"], &userHome)
	if secret != "" || home != os.Getenv("PANGU_CODEX_HOME") || userHome != home {
		t.Fatalf("继承了个人环境或配置: secretPresent=%t home=%q userHome=%q", secret != "", home, userHome)
	}
	t.Setenv("PANGU_CODEX_HOME", "")
	if _, err := Execute(context.Background(), codexInput("")); err == nil || workflow.CanRetry(err) {
		t.Fatal("缺少专用配置应拒绝且不重试", err)
	}
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
	t.Setenv("CODEX_TEST_WORKSPACE", t.TempDir())
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
	t.Setenv("CODEX_TEST_WORKSPACE", "relative/path")
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
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "等待子进程取消") {
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
