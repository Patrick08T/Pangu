package agent

import (
	"context"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func Execute(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	if err := input.Node.Validate(); err != nil {
		return workflow.NodeOutput{}, err
	}
	var output workflow.NodeOutput
	var err error
	if input.Node.Executor == "codex" {
		output, err = executeCodex(ctx, input)
	} else {
		output, err = executeEino(ctx, input)
	}
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	if err := ctx.Err(); err != nil {
		return workflow.NodeOutput{}, err
	}
	if output.Text == "" || len(output.Text) > 16384 {
		return workflow.NodeOutput{}, fmt.Errorf("Agent 输出为空或超过 16 KiB")
	}
	return output, nil
}
