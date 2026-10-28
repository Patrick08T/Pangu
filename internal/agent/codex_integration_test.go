package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestNativeCodexClassifiesLocalAuthenticationFailure(t *testing.T) {
	if os.Getenv("PANGU_CODEX_CONFIG_CHECK") != "1" {
		t.Skip("显式启用本机 CLI 与本地模拟端点，无付费请求")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"message":"Invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}))
	defer server.Close()
	home, dir := t.TempDir(), t.TempDir()
	dir = filepath.Join(dir, "project.with.dots")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_CODEX_HOME", home)
	config := fmt.Sprintf("model_provider=\"local\"\n[model_providers.local]\nname=\"local\"\nbase_url=%q\nwire_api=\"responses\"\nrequires_openai_auth=false\nrequest_max_retries=0\nstream_max_retries=0\n", server.URL)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := Execute(ctx, workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "one", Executor: "codex", Model: "gpt-5.4", Instruction: "本地模拟认证测试"}})
	if workflow.FailureKind(err) != "authentication" || workflow.CanRetry(err) {
		t.Fatalf("认证失败仍会重试: %v", err)
	}
}

func TestNativeCodexIsolationOverridesDedicatedFeatures(t *testing.T) {
	if os.Getenv("PANGU_CODEX_CONFIG_CHECK") != "1" {
		t.Skip("显式启用本机 CLI 无模型配置检查")
	}
	home := t.TempDir()
	t.Setenv("PANGU_CODEX_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[features]\nmulti_agent=true\nmulti_agent_v2=true\napps=true\nplugins=true\nhooks=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{}
	for _, arg := range codexIsolationArgs(t.TempDir()) {
		if arg != "--strict-config" && arg != "--ignore-rules" {
			args = append(args, arg)
		}
	}
	args = append(args, "features", "list")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Getenv("PANGU_CODEX_BINARY"), args...)
	cmd.Env = codexEnvironment()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI 配置检查失败: %v %s", err, out)
	}
	want := map[string]string{"multi_agent": "false", "multi_agent_v2": "false", "apps": "false", "plugins": "false", "hooks": "false", "skip_host_skill_discovery": "true"}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if expected, ok := want[fields[0]]; ok {
			if fields[len(fields)-1] != expected {
				t.Fatalf("专用配置越过强制隔离: %s", line)
			}
			delete(want, fields[0])
		}
	}
	if len(want) != 0 {
		t.Fatal("CLI 缺少必要的隔离功能", want)
	}
}

func TestNativeCodexWritesWorkspace(t *testing.T) {
	if os.Getenv("PANGU_CODEX_REAL") != "1" {
		t.Skip("设置 PANGU_CODEX_REAL=1，使用已登录的原生 Codex")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	input := workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "smoke", Executor: "codex",
		Instruction: "这是临时测试项目，只使用 Git，不使用 P4。请使用工具在当前项目创建 proof.txt，内容精确为 pangu-codex-ok。只完成此项，不提交 Git，最终用中文简短报告。"}}
	out, err := Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "proof.txt"))
	if err != nil || strings.TrimSpace(string(data)) != "pangu-codex-ok" {
		t.Fatalf("文件验证失败: %v %q", err, data)
	}
	t.Logf("原生 Codex 实际写入成功，最终输出: %s", out.Text)
}
