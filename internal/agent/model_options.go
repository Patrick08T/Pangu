package agent

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func validateCodexOptions(ctx context.Context, node workflow.Node) error {
	if err := node.ValidateModelOptions(); err != nil {
		return err
	}
	if !node.HasModelOptions() {
		return nil
	}
	models, err := CodexModels(ctx)
	if err != nil {
		return err
	}
	for _, model := range models {
		if model.ID != node.Model {
			continue
		}
		if node.ReasoningEffort != "" && !slices.Contains(model.ReasoningLevels, node.ReasoningEffort) {
			return errors.New("所选模型不支持该推理强度")
		}
		if node.Fast != nil && *node.Fast && !model.Fast {
			return errors.New("所选模型不支持 Fast")
		}
		return nil
	}
	return errors.New("所选模型不在 Worker 当前模型目录中")
}

func codexExecutionSettings(ctx context.Context, input workflow.NodeInput) (string, string, []string, error) {
	binary, workspace, err := codexPaths(input.Workspace)
	if err == nil {
		err = checkCodexPolicy(ctx, binary)
	}
	if err == nil {
		err = validateCodexOptions(ctx, input.Node)
	}
	return binary, workspace, codexModelArgs(input.Node), err
}

func codexModelArgs(node workflow.Node) []string {
	var args []string
	if node.Model != "" {
		args = append(args, "--model="+node.Model)
	}
	if node.ReasoningEffort != "" {
		args = append(args, "-c", "model_reasoning_effort="+strconv.Quote(node.ReasoningEffort))
	}
	if node.Fast != nil {
		tier := "default"
		if *node.Fast {
			tier = "priority"
		}
		args = append(args, "-c", "service_tier="+strconv.Quote(tier))
		if *node.Fast {
			args = append(args, "--enable", "fast_mode")
		}
	}
	return args
}
