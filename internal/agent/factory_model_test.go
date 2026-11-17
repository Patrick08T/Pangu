package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino/schema"
)

func TestFactoryCodexSelectionOverridesConnectionDefault(t *testing.T) {
	setupCodex(t)
	input, _, _ := codexConnectionInput(t, "bridge")
	fast := false
	input.Node.Model, input.Node.ReasoningEffort, input.Node.Fast = "options-bridge", "high", &fast
	ctx := context.Background()
	if !Preflight(ctx, workflow.Definition{ProjectID: input.ProjectID, Nodes: []workflow.Node{input.Node}}).Ready {
		t.Fatal("选定模型无法通过账号连接预检")
	}
	chat, err := einoModel(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := chat.Generate(ctx, []*schema.Message{schema.UserMessage("检查参数")})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"--model=options-bridge", `model_reasoning_effort="high"`, `service_tier="default"`, "--sandbox read-only"} {
		if !strings.Contains(got.Content, expected) {
			t.Fatal("选项未传递或权限扩大", expected, got.Content)
		}
	}
}
