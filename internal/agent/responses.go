package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const responsesHistoryKey = "openai_responses_output"

type responsesModel struct {
	config *openai.ChatModelConfig
	effort string
	tools  []*schema.ToolInfo
}

type responsesTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
	Strict      bool   `json:"strict"`
}

func newResponsesModel(config *openai.ChatModelConfig, effort string) *responsesModel {
	return &responsesModel{config: config, effort: effort}
}

func (m *responsesModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	copy := *m
	copy.tools = append([]*schema.ToolInfo(nil), tools...)
	return &copy, nil
}

func (m *responsesModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	options := model.GetCommonOptions(&model.Options{Tools: m.tools}, opts...)
	input, err := responsesInput(messages)
	if err != nil {
		return nil, err
	}
	tools, err := responsesTools(options.Tools)
	if err != nil {
		return nil, err
	}
	request := struct {
		Model     string            `json:"model"`
		Input     []any             `json:"input"`
		Tools     []responsesTool   `json:"tools,omitempty"`
		Store     bool              `json:"store"`
		Include   []string          `json:"include"`
		Reasoning map[string]string `json:"reasoning"`
	}{
		Model: m.config.Model, Input: input, Tools: tools, Include: []string{"reasoning.encrypted_content"},
		Reasoning: map[string]string{"effort": m.effort},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(m.config.BaseURL, "/")+"/responses", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := m.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenAI Responses API 返回 HTTP %d", res.StatusCode)
	}
	var reply struct {
		Status string            `json:"status"`
		Output []json.RawMessage `json:"output"`
	}
	if err = json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return nil, err
	}
	if reply.Status != "completed" {
		return nil, errors.New("OpenAI Responses API 未返回完整响应")
	}
	return responsesOutput(reply.Output)
}

func (m *responsesModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reply, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{reply}), nil
}

func responsesTools(infos []*schema.ToolInfo) ([]responsesTool, error) {
	var tools []responsesTool
	for _, info := range infos {
		var parameters any = map[string]any{"type": "object", "properties": map[string]any{}}
		if info.ParamsOneOf != nil {
			value, err := info.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return nil, err
			}
			parameters = value
		}
		tools = append(tools, responsesTool{Type: "function", Name: info.Name, Description: info.Desc, Parameters: parameters})
	}
	return tools, nil
}

func responsesInput(messages []*schema.Message) ([]any, error) {
	input := make([]any, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case schema.System, schema.User:
			input = append(input, map[string]string{"role": string(msg.Role), "content": msg.Content})
		case schema.Assistant:
			if history, ok := msg.Extra[responsesHistoryKey].([]json.RawMessage); ok {
				for _, item := range history {
					input = append(input, item)
				}
			} else if len(msg.ToolCalls) == 0 {
				input = append(input, map[string]string{"role": "assistant", "content": msg.Content})
			} else {
				return nil, errors.New("缺少 Responses API 工具历史")
			}
		case schema.Tool:
			input = append(input, map[string]string{"type": "function_call_output", "call_id": msg.ToolCallID, "output": msg.Content})
		default:
			return nil, errors.New("Responses API 输入包含不支持的消息角色")
		}
	}
	return input, nil
}

func responsesOutput(items []json.RawMessage) (*schema.Message, error) {
	msg := schema.AssistantMessage("", nil)
	var content strings.Builder
	for _, raw := range items {
		var item struct {
			Type      string `json:"type"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		switch item.Type {
		case "reasoning":
		case "function_call":
			if item.CallID == "" || item.Name == "" || !json.Valid([]byte(item.Arguments)) {
				return nil, errors.New("Responses API 返回无效工具调用")
			}
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{ID: item.CallID, Type: "function", Function: schema.FunctionCall{Name: item.Name, Arguments: item.Arguments}})
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					content.WriteString(part.Text)
				}
			}
		default:
			return nil, errors.New("Responses API 返回不支持的输出项")
		}
	}
	msg.Content = content.String()
	if msg.Content == "" && len(msg.ToolCalls) == 0 {
		return nil, errors.New("Responses API 未返回文本或工具调用")
	}
	msg.Extra = map[string]any{responsesHistoryKey: items}
	return msg, nil
}
