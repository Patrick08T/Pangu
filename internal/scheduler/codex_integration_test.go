package scheduler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"context"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func TestHatchetNativeCodex(t *testing.T) {
	if os.Getenv("PANGU_CODEX_REAL") != "1" {
		t.Skip("设置 PANGU_CODEX_REAL=1，使用已登录的原生 Codex")
	}
	_, backend, h := integrationSetup(t)
	project := registerTestProject(t, backend)
	d := workflow.Definition{ProjectID: project.ID, Input: "原生 Codex 集成验证", Nodes: []workflow.Node{
		{ID: "source", Executor: "eino", Model: "source", Instruction: "生成计划"},
		{ID: "code", Executor: "codex", DependsOn: []string{"source"},
			Instruction: "这是临时测试 Git 项目，不使用 P4。请用工具创建 proof.txt，内容精确等于 parents.source.text。只完成此项，不提交 Git，最终用中文简短报告。"},
		{ID: "summary", Executor: "eino", Model: "summary", Instruction: "汇总结果", DependsOn: []string{"code"}},
	}}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	request(t, h, "POST", "/runs", d, 202, &accepted)
	out := waitRun(t, h, accepted.RunID)
	record, err := workspace.Get(context.Background(), accepted.RunID)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "show", record.HeadCommit+":proof.txt")
	cmd.Dir = filepath.Join(record.Directory, "base")
	data, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(data)) != "source output" || len(out.Nodes) != 3 {
		t.Fatalf("原生混合编排结果不符: %v %q nodes=%d", err, data, len(out.Nodes))
	}
	if _, err := os.Stat(filepath.Join(project.Source, "proof.txt")); !os.IsNotExist(err) {
		t.Fatal("原始项目被修改")
	}
	t.Logf("API → Hatchet → Eino → 原生 Codex 写文件 → Eino 通过，运行 ID=%s", accepted.RunID)
}
