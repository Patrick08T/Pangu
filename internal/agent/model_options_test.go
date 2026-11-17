package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino/schema"
)

func TestModelDirectoryPreservesReasoningAndFastCapabilities(t *testing.T) {
	raw := []byte(`{"models":[{"slug":"test","visibility":"list","default_reasoning_level":"medium","supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"}],"service_tiers":[{"id":"priority","name":"Fast"}]}]}`)
	items, err := codexModelChoices(raw)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(items)
	for _, expected := range []string{`"reasoning_levels":["low","medium"]`, `"default_reasoning":"medium"`, `"fast":true`} {
		if !strings.Contains(string(data), expected) {
			t.Errorf("模型能力丢失 %s：%s", expected, data)
		}
	}
}

func TestCodexAppliesReasoningAndExplicitFastServiceTier(t *testing.T) {
	setupCodex(t)
	for _, fast := range []bool{false, true} {
		input := codexInput("options")
		input.Node.ReasoningEffort, input.Node.Fast = "high", &fast
		out, err := Execute(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Args []string `json:"args"`
		}
		if err = json.Unmarshal([]byte(out.Text), &got); err != nil {
			t.Fatal(err)
		}
		args := strings.Join(got.Args, " ")
		tier := "default"
		if fast {
			tier = "priority"
		}
		if !strings.Contains(args, `model_reasoning_effort="high"`) || !strings.Contains(args, `service_tier="`+tier+`"`) {
			t.Fatalf("运行选项未传递到程序：%s", args)
		}
	}
}

func TestCodexRejectsUnsupportedOptionsBeforeExecutingModel(t *testing.T) {
	setupCodex(t)
	input := codexInput("options")
	input.Node.ReasoningEffort = "xhigh"
	if _, err := Execute(context.Background(), input); workflow.FailureKind(err) != "parameter" || workflow.CanRetry(err) {
		t.Fatal("不支持的选项应直接拒绝", err)
	}
	valid := input.Node
	valid.ID, valid.ReasoningEffort = "first", "high"
	if Preflight(context.Background(), workflow.Definition{Nodes: []workflow.Node{valid, input.Node}}).Ready {
		t.Fatal("同模型去重遗漏不同运行选项")
	}
}

func TestEinoBridgePassesModelOptionsToCodex(t *testing.T) {
	setupCodex(t)
	input := codexInput("options-bridge")
	fast := false
	input.Node.ReasoningEffort, input.Node.Fast = "high", &fast
	got, err := codexTurn(context.Background(), input, []*schema.Message{schema.UserMessage("检查参数")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, `model_reasoning_effort="high"`) || !strings.Contains(got.Content, `service_tier="default"`) {
		t.Fatal("桥接丢失运行选项", got.Content)
	}
}

func TestLocalPerforceCodexDoesNotRequireGitRepository(t *testing.T) {
	setupCodex(t)
	input := codexInput("options")
	input.Revision = &workflow.Revision{LocalPerforce: true, Changelist: 12}
	out, err := Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Args []string `json:"args"`
	}
	if err = json.Unmarshal([]byte(out.Text), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(got.Args, " "), "--skip-git-repo-check") {
		t.Fatal("本地 P4 被强制要求 Git 仓库", got.Args)
	}
}
