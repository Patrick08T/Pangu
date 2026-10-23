package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestNativeCodexWritesWorkspace(t *testing.T) {
	if os.Getenv("PANGU_CODEX_REAL") != "1" {
		t.Skip("设置 PANGU_CODEX_REAL=1，使用已登录的原生 Codex")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	t.Setenv("PANGU_CODEX_WORKSPACE", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	input := workflow.NodeInput{Node: workflow.Node{ID: "smoke", Executor: "codex",
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
