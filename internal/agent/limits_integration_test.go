package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func nativePolicyWorkspace(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	if os.Getenv("PANGU_CODEX_CONFIG_CHECK") != "1" {
		t.Skip("显式启用本机 CLI 和本地模拟端点，不调用外部模型")
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	root := filepath.Join("..", "..", ".artifacts")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	home, err := os.MkdirTemp(root, "codex-home-")
	if err != nil {
		t.Fatal(err)
	}
	home, _ = filepath.Abs(home)
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	dir := t.TempDir()
	t.Setenv("PANGU_CODEX_HOME", home)
	catalog := `{"models":[{"slug":"gpt-5.4","display_name":"local","base_instructions":"Local test only","supported_reasoning_levels":[],"shell_type":"shell_command","visibility":"list","supported_in_api":true,"priority":0,"support_verbosity":false,"apply_patch_tool_type":"freeform","truncation_policy":{"mode":"tokens","limit":10000},"experimental_supported_tools":[],"tool_mode":"direct"}]}`
	if err := os.WriteFile(filepath.Join(home, "models.json"), []byte(catalog), 0600); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("model_catalog_json=%q\nmodel_provider=\"local\"\n[model_providers.local]\nname=\"local\"\nbase_url=%q\nwire_api=\"responses\"\nrequires_openai_auth=false\nrequest_max_retries=0\nstream_max_retries=0\n", filepath.ToSlash(filepath.Join(home, "models.json")), server.URL)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	return dir
}

func nativeToolResponse(w http.ResponseWriter, item map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	events := []map[string]any{
		{"type": "response.created", "response": map[string]any{"id": "local"}},
		{"type": "response.output_item.done", "item": item},
		{"type": "response.completed", "response": map[string]any{"id": "local", "usage": map[string]any{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0}}},
	}
	for _, event := range events {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
	}
}

func TestNativeCodexForcesDirectToolsDespiteModelCodeMode(t *testing.T) {
	var observed atomic.Bool
	dir := nativePolicyWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tools []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		tools, _ := json.Marshal(body.Tools)
		observed.Store(strings.Contains(string(tools), `"apply_patch"`) && !strings.Contains(string(tools), `"code_mode"`))
		w.WriteHeader(http.StatusUnauthorized)
	})
	home := os.Getenv("PANGU_CODEX_HOME")
	catalog := `{"models":[{"slug":"local-code-mode","display_name":"local","base_instructions":"Local test only","supported_reasoning_levels":[],"shell_type":"shell_command","visibility":"list","supported_in_api":true,"priority":0,"support_verbosity":false,"apply_patch_tool_type":"freeform","truncation_policy":{"mode":"tokens","limit":10000},"experimental_supported_tools":[],"tool_mode":"code_mode_only"}]}`
	if err := os.WriteFile(filepath.Join(home, "models.json"), []byte(catalog), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "config.toml")
	config, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	extra := "features.code_mode=true\n"
	if err := os.WriteFile(path, append([]byte(extra), config...), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := append([]string{"exec", "--ephemeral", "--json", "--model=local-code-mode", "--cd", dir}, codexIsolationArgs(dir)...)
	cmd := exec.CommandContext(ctx, os.Getenv("PANGU_CODEX_BINARY"), append(args, "-")...)
	cmd.Dir, cmd.Stdin = dir, strings.NewReader("本地工具模式测试")
	cmd.Env = append(codexEnvironment(), "PANGU_CODEX_TRACE=1")
	out := &diagnostic.Buffer{Limit: 8192}
	log := &diagnostic.Buffer{Limit: 8192}
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = out, log, time.Second
	err = runCodexProcess(ctx, cmd, "")
	if err == nil || !observed.Load() {
		t.Fatalf("模型配置仍可开启异步 Code Mode: %v %s %s", err, out.Data.String(), log.Data.String())
	}
}

func TestNativeCodexRunsBeyondTenToolsWithCorrelatedTrace(t *testing.T) {
	var calls atomic.Int32
	dir := nativePolicyWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n > 12 {
			nativeFinalResponse(w)
			return
		}
		nativeToolResponse(w, map[string]any{"type": "custom_tool_call", "call_id": fmt.Sprint(n), "name": "apply_patch",
			"input": fmt.Sprintf("*** Begin Patch\n*** Add File: proof-%d.txt\n+ok\n*** End Patch", n)})
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var events []trace.Event
	ctx = trace.WithSink(ctx, func(_ context.Context, event trace.Event) error { events = append(events, event); return nil })
	_, err := Execute(ctx, workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "trace", Executor: "codex", Model: "gpt-5.4", Instruction: "本地追踪测试"}})
	if err != nil {
		t.Fatalf("超过十次调用失败: %v requests=%d", err, calls.Load())
	}
	for n := 1; n <= 12; n++ {
		if data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("proof-%d.txt", n))); err != nil || strings.TrimSpace(string(data)) != "ok" {
			t.Fatalf("第 %d 次未执行: %v", n, err)
		}
	}
	models, tools := map[string]bool{}, 0
	for _, event := range events {
		if event.Kind == "model" {
			models[event.SpanID] = true
		}
	}
	for _, event := range events {
		if event.Kind == "tool" && event.Phase == "end" {
			tools++
			if event.Status != "succeeded" || !models[event.ParentID] {
				t.Fatalf("调用关联或结果缺失: %+v", event)
			}
		}
	}
	if tools != 12 || len(models) != 13 {
		t.Fatalf("trace 不完整: tools=%d models=%d events=%d", tools, len(models), len(events))
	}
}

func nativeFinalResponse(w http.ResponseWriter) {
	nativeToolResponse(w, map[string]any{"type": "message", "role": "assistant", "content": []map[string]string{{"type": "output_text", "text": "完成"}}})
}

func TestNativeCodexCancellationStillStopsDescendants(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Job 后代终止场景")
	}
	dir := nativePolicyWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		args, _ := json.Marshal(map[string]any{"cmd": "Set-Content -LiteralPath started.txt -Value started; Start-Sleep -Seconds 4; Set-Content -LiteralPath orphan.txt -Value leaked", "yield_time_ms": 10000})
		nativeToolResponse(w, map[string]any{"type": "function_call", "call_id": "cancel", "name": "exec_command", "arguments": string(args)})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Execute(ctx, workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "cancel", Executor: "codex", Model: "gpt-5.4", Instruction: "本地取消测试"}})
		done <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "started.txt")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("命令没有启动")
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("取消后仍成功")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("进程未停止")
	}
	time.Sleep(4 * time.Second)
	if _, err := os.Stat(filepath.Join(dir, "orphan.txt")); !os.IsNotExist(err) {
		t.Fatal("取消后仍有子进程写入", err)
	}
}

func TestNativeCodexToolCanRunBeyondThirtySeconds(t *testing.T) {
	var calls atomic.Int32
	var dir string
	dir = nativePolicyWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if _, err := os.Stat(filepath.Join(dir, "completed.txt")); err == nil {
			nativeFinalResponse(w)
			return
		}
		name, args := "exec_command", map[string]any{"cmd": "Start-Sleep -Seconds 32; Set-Content -LiteralPath completed.txt -Value done", "yield_time_ms": 1000}
		if n > 1 {
			body, _ := io.ReadAll(r.Body)
			match := regexp.MustCompile(`session ID (\d+)`).FindSubmatch(body)
			if len(match) != 2 {
				t.Errorf("命令没有返回可轮询会话: %s", body)
				nativeFinalResponse(w)
				return
			}
			id, _ := strconv.Atoi(string(match[1]))
			name, args = "write_stdin", map[string]any{"session_id": id, "chars": "", "yield_time_ms": 1000}
		}
		encoded, _ := json.Marshal(args)
		nativeToolResponse(w, map[string]any{"type": "function_call", "call_id": fmt.Sprintf("long-%d", n), "name": name, "arguments": string(encoded)})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	_, err := Execute(ctx, workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "long", Executor: "codex", Model: "gpt-5.4", Instruction: "本地长工具测试"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "completed.txt")); err != nil {
		t.Fatal("超过 30 秒工具未完成", err)
	}
}

func TestNativeCodexReadOnlySandboxRejectsPatch(t *testing.T) {
	var calls atomic.Int32
	var rejected atomic.Bool
	dir := nativePolicyWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			nativeToolResponse(w, map[string]any{"type": "custom_tool_call", "call_id": "readonly", "name": "apply_patch", "input": "*** Begin Patch\n*** Add File: forbidden.txt\n+must not exist\n*** End Patch"})
			return
		}
		var body struct {
			Input []struct{ Output json.RawMessage }
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, item := range body.Input {
			if strings.Contains(string(item.Output), "read-only sandbox") {
				rejected.Store(true)
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := Execute(ctx, workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "readonly", Executor: "codex", Model: "gpt-5.4", Instruction: "本地只读测试", ReadOnly: true}})
	if workflow.FailureKind(err) != "authentication" || !rejected.Load() {
		t.Fatalf("未收到原生沙箱的只读拒绝: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "forbidden.txt")); !os.IsNotExist(err) {
		t.Fatal("只读节点发生写入", err)
	}
}
