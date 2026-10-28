package agent

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"os"
	"strings"
	"testing"
)

func TestFactoryUsesEinoCodexBridgeWithoutTools(t *testing.T) {
	setupCodex(t)
	input, _, _ := codexConnectionInput(t, "factory")
	in := factory.Input{ID: uuid.NewString(), Revision: 1, Message: "只生成候选", Mode: "build", ConnectionVersion: input.Node.ConnectionVersion}
	d, _, err := factory.Definition(factory.Session{ProjectID: input.ProjectID}, in, factory.Context{Connections: []catalog.ConnectionChoice{{ID: in.ConnectionVersion, Model: "factory"}}})
	if err != nil {
		t.Fatal(err)
	}
	input.Node, input.Input = d.Nodes[0], d.Input
	out, err := Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := factory.Parse(out.Text)
	if err != nil || reply.Message != "请补充交付目标" || reply.Proposal != nil {
		t.Fatal("Codex 适配未返回合法候选", err)
	}
	if err = workflow.ValidateOutput(input.Node, &out); err != nil {
		t.Fatal(err)
	}
}

func factoryCodexHelper(input workflow.NodeInput, args map[string]string) int {
	var prompt struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
		Tools []any `json:"tools"`
	}
	if json.Unmarshal([]byte(input.Input), &prompt) != nil || len(prompt.Tools) != 0 || len(prompt.Messages) < 2 {
		return 61
	}
	if args["--sandbox"] != "read-only" || os.Getenv("PANGU_CODEX_MODEL_ONLY") != "1" {
		return 62
	}
	if !strings.Contains(prompt.Messages[0].Content, "Pangu 内置工厂助手") || !strings.Contains(prompt.Messages[len(prompt.Messages)-1].Content, "Workflow 构建与优化") {
		return 63
	}
	out, _ := json.Marshal(map[string]any{"content": `{"message":"请补充交付目标"}`, "tool_calls": []any{}})
	if os.WriteFile(args["--output-last-message"], out, 0600) != nil {
		return 64
	}
	return 0
}
