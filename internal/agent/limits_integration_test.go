package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
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
	cmd.Env = append(codexEnvironment(), "PANGU_CODEX_TOOL_CALLS=10")
	out := &diagnostic.Buffer{Limit: 8192}
	log := &diagnostic.Buffer{Limit: 8192}
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = out, log, time.Second
	err = runCodexProcess(ctx, cmd, "")
	if err == nil || !observed.Load() {
		t.Fatalf("模型配置仍可开启异步 Code Mode: %v %s %s", err, out.Data.String(), log.Data.String())
	}
}

func TestNativeCodexDeniesEleventhToolBeforeWriting(t *testing.T) {
	var calls atomic.Int32
	dir := nativePolicyWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/responses") {
			http.NotFound(w, r)
			return
		}
		n := calls.Add(1)
		if n == 2 {
			var body struct {
				Input []struct {
					Type   string
					Output json.RawMessage
				}
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			for _, item := range body.Input {
				if strings.Contains(item.Type, "tool_call_output") || item.Type == "function_call_output" {
					t.Logf("首次工具响应: %s", item.Output)
				}
			}
		}
		if n > 12 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		nativeToolResponse(w, map[string]any{"type": "custom_tool_call", "call_id": fmt.Sprint(n), "name": "apply_patch",
			"input": fmt.Sprintf("*** Begin Patch\n*** Add File: proof-%d.txt\n+ok\n*** End Patch", n)})
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := Execute(ctx, workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "budget", Executor: "codex", Model: "gpt-5.4", Instruction: "本地限额测试"}})
	if workflow.FailureKind(err) != "tool_limit" || workflow.CanRetry(err) {
		t.Fatalf("超额未停止: %v，本地模型请求数=%d", err, calls.Load())
	}
	for n := 1; n <= 10; n++ {
		if data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("proof-%d.txt", n))); err != nil || strings.TrimSpace(string(data)) != "ok" {
			t.Fatalf("第%d次未实际执行: %s %v", n, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "proof-11.txt")); !os.IsNotExist(err) {
		t.Fatal("第11次调用已写入", err)
	}
}

func TestNativeCodexToolDeadlineStopsDescendants(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Job 后代终止场景")
	}
	dir := nativePolicyWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		args, _ := json.Marshal(map[string]any{"cmd": "Set-Content -LiteralPath started.txt -Value started; Start-Sleep -Seconds 35; Set-Content -LiteralPath orphan.txt -Value leaked", "timeout_ms": 60000})
		nativeToolResponse(w, map[string]any{"type": "function_call", "call_id": "timeout", "name": "exec_command", "arguments": string(args)})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	started := time.Now()
	_, err := Execute(ctx, workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "deadline", Executor: "codex", Model: "gpt-5.4", Instruction: "本地时限测试"}})
	if workflow.FailureKind(err) != "tool_timeout" || workflow.CanRetry(err) || time.Since(started) > 35*time.Second {
		t.Fatalf("单工具30秒时限未强制: %v, 耗时%v", err, time.Since(started))
	}
	if _, err := os.Stat(filepath.Join(dir, "started.txt")); err != nil {
		t.Fatal("未证明工具子进程实际开始执行", err)
	}
	time.Sleep(max(0, 37*time.Second-time.Since(started)))
	if _, err := os.Stat(filepath.Join(dir, "orphan.txt")); !os.IsNotExist(err) {
		t.Fatal("超时后仍有子进程写入", err)
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
