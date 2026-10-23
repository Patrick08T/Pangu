package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func executeEino(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	var session *research.Session
	if input.Node.Role != "" {
		var err error
		session, err = research.New(input)
		if err != nil {
			return workflow.NodeOutput{}, &workflow.Failure{Kind: "parameter", Cause: err}
		}
	}
	config, err := modelConfig(ctx, input.ProjectID, input.Node)
	if err != nil {
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "configuration", Cause: err}
	}
	chat, err := openai.NewChatModel(ctx, config)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	settings := &adk.ChatModelAgentConfig{
		Name: input.Node.ID, Description: input.Node.Description,
		Instruction: outputInstruction(input.Node), Model: chat, MaxIterations: 1,
		GenModelInput: literalInstruction,
	}
	if session != nil {
		settings.ToolsConfig = session.Tools()
		settings.Instruction = session.Instruction() + "\n补充任务要求：" + input.Node.Instruction
		settings.MaxIterations = input.Node.EffectivePolicy().ToolCalls + 1
	}
	a, err := adk.NewChatModelAgent(ctx, settings)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	out, err := runEino(ctx, a, input)
	if session != nil {
		result, reportErr := session.Finish(out)
		if err != nil {
			return result, err
		}
		return result, reportErr
	}
	return out, err
}

func runEino(ctx context.Context, a adk.Agent, input workflow.NodeInput) (workflow.NodeOutput, error) {
	payload, err := json.Marshal(struct {
		Input      string                         `json:"input"`
		Parents    map[string]workflow.NodeOutput `json:"parents"`
		Parameters map[string]any                 `json:"parameters,omitempty"`
	}{input.Input, input.Parents, input.Parameters})
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

func modelConfig(ctx context.Context, project string, node workflow.Node) (*openai.ChatModelConfig, error) {
	config := &openai.ChatModelConfig{APIKey: os.Getenv("OPENAI_API_KEY"), BaseURL: os.Getenv("OPENAI_BASE_URL"), Model: node.Model, Timeout: 2 * time.Minute}
	config.Timeout = min(config.Timeout, time.Duration(node.EffectivePolicy().TimeoutSeconds)*time.Second)
	if node.ConnectionVersion != "" {
		connection, key, err := catalog.ResolveConnection(ctx, project, node.ConnectionVersion)
		if err != nil {
			return nil, err
		}
		if connection.Model != node.Model {
			return nil, fmt.Errorf("节点模型与连接版本不一致")
		}
		config.APIKey, config.BaseURL = key, connection.BaseURL
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("模型凭据尚未配置")
	}
	config.HTTPClient = &http.Client{Timeout: config.Timeout, Transport: &http.Transport{TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: config.Timeout, MaxResponseHeaderBytes: 64 * 1024, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return config, nil
}
