package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func executeEino(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return workflow.NodeOutput{}, fmt.Errorf("缺少 OPENAI_API_KEY")
	}
	chat, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: key, BaseURL: os.Getenv("OPENAI_BASE_URL"), Model: input.Node.Model,
		Timeout: 2 * time.Minute,
	})
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: input.Node.ID, Description: input.Node.Description,
		Instruction: input.Node.Instruction, Model: chat, MaxIterations: 1,
		GenModelInput: literalInstruction,
	})
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	payload, err := json.Marshal(struct {
		Input   string                         `json:"input"`
		Parents map[string]workflow.NodeOutput `json:"parents"`
	}{input.Input, input.Parents})
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: a})
	iter := runner.Query(ctx, string(payload))
	var output workflow.NodeOutput
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return workflow.NodeOutput{}, event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			message, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				return workflow.NodeOutput{}, err
			}
			if message != nil && message.Role == schema.Assistant {
				output.Text = message.Content
			}
		}
	}
	return output, nil
}

func literalInstruction(_ context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {
	// JSON 指令中的花括号是原始内容，不作为 Eino 模板变量解析。
	return append([]adk.Message{schema.SystemMessage(instruction)}, input.Messages...), nil
}
