package agent

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func outputInstruction(node workflow.Node) string {
	if node.OutputSchema == nil {
		return node.Instruction
	}
	data, _ := json.Marshal(node.OutputSchema)
	return node.Instruction + "\n最终输出必须是符合以下结构的 JSON 对象，不添加 Markdown 围栏或未声明字段：" + string(data)
}

func Execute(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	if err := input.Node.Validate(); err != nil {
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "parameter", Cause: err}
	}
	input.Node.Instruction += input.Node.Handoff.Instruction()
	var output workflow.NodeOutput
	var err error
	if input.Node.Executor == "codex" {
		output, err = executeCodex(ctx, input)
	} else {
		output, err = executeEino(ctx, input)
	}
	if err != nil {
		return output, classify(err)
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	err = workflow.ValidateOutput(input.Node, &output)
	if err != nil {
		err = &workflow.Failure{Kind: "output", Cause: err}
	}
	return output, err
}
