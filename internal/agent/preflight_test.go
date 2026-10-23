package agent

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestPreflightChecksOnlyRequestedExecutors(t *testing.T) {
	setupCodex(t)
	d := workflow.Definition{Nodes: []workflow.Node{{ID: "code", Executor: "codex", Instruction: "任务"}}}
	result := Preflight(context.Background(), d)
	if !result.Ready {
		t.Fatalf("纯 Codex 不应要求 Eino 密钥: %+v", result)
	}
	d.Nodes = append(d.Nodes, workflow.Node{ID: "model", Executor: "eino", Instruction: "任务", Model: "test"})
	if Preflight(context.Background(), d).Ready {
		t.Fatal("缺少 Eino 配置仍报告就绪")
	}
	t.Setenv("OPENAI_API_KEY", "test-key")
	if !Preflight(context.Background(), d).Ready {
		t.Fatal("配置有效时应就绪")
	}
	t.Setenv("PANGU_CODEX_BINARY", "missing")
	if Preflight(context.Background(), d).Ready {
		t.Fatal("检查缓存了程序路径")
	}
}
