package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type codexModel struct {
	input workflow.NodeInput
	tools []*schema.ToolInfo
}

func (m *codexModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	copy := *m
	copy.tools = append([]*schema.ToolInfo(nil), tools...)
	return &copy, nil
}

func (m *codexModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	o := model.GetCommonOptions(&model.Options{Tools: m.tools}, opts...)
	if o.Model != nil || o.Temperature != nil || o.TopP != nil || o.MaxTokens != nil || len(o.Stop) != 0 || o.ToolChoice != nil || len(o.AllowedToolNames) != 0 || o.AgenticToolChoice != nil || len(o.DeferredTools) != 0 || o.ToolSearchTool != nil {
		return nil, errors.New("Codex 模型桥接仅支持工具定义；模型由固定连接版本指定")
	}
	c, _, err := catalog.ResolveConnection(ctx, m.input.ProjectID, m.input.Node.ConnectionVersion)
	if err != nil {
		return nil, err
	}
	if c.Provider != "codex" || c.Model != m.input.Node.Model {
		return nil, errors.New("Codex 模型连接与节点不一致")
	}
	return codexTurn(ctx, m.input, messages, o.Tools)
}

func (m *codexModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// shortcut: CLI 完成后返回整段消息；需要逐 token 展示时再接事件流。
	message, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

func codexTurn(ctx context.Context, input workflow.NodeInput, messages []*schema.Message, tools []*schema.ToolInfo) (*schema.Message, error) {
	prompt, err := codexPrompt(messages, tools)
	if err != nil {
		return nil, err
	}
	root := os.Getenv("PANGU_WORKSPACE_ROOT")
	if !filepath.IsAbs(root) {
		return nil, errors.New("Codex 模型桥接需要 Worker 私有工作区绝对路径")
	}
	dir, err := os.MkdirTemp(root, "eino-codex-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "response.json")
	if err := os.WriteFile(path, []byte(codexResponseSchema), 0600); err != nil {
		return nil, err
	}
	input.Workspace, input.Input, input.Parents, input.Parameters = dir, prompt, nil, nil
	input.Node = workflow.Node{ID: input.Node.ID, Executor: "codex", Model: input.Node.Model, ReadOnly: true,
		Instruction: "你是 Eino 的模型后端。根据 input.messages 中完整对话生成下一条 assistant 回复，遵守其中 system 指令。input.tools 是仅可请求 Eino 执行的工具定义，不是本机工具。需要工具时只返回一个 tool_calls 项，arguments 为 JSON 对象字符串；普通回答填写 content 且 tool_calls 为空数组。不要调用任何原生工具，不读取本机文件。严格按输出结构返回 JSON。",
		Policy:      &workflow.Policy{}}
	result, err := runCodex(ctx, input, path, nil)
	if err != nil {
		return nil, err
	}
	return codexReply(result.Text, tools)
}

func codexPrompt(messages []*schema.Message, tools []*schema.ToolInfo) (string, error) {
	definitions := make([]map[string]any, 0, len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		if tool == nil || tool.Name == "" || seen[tool.Name] {
			return "", errors.New("Eino 工具定义缺失或重名")
		}
		seen[tool.Name] = true
		params, err := tool.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return "", err
		}
		definitions = append(definitions, map[string]any{"name": tool.Name, "description": tool.Desc, "parameters": params})
	}
	data, err := json.Marshal(map[string]any{"messages": messages, "tools": definitions})
	if err != nil {
		return "", err
	}
	if len(data) > 1024*1024 {
		return "", errors.New("Eino 对话与工具定义超过 1 MiB")
	}
	return string(data), nil
}

func codexReply(text string, tools []*schema.ToolInfo) (*schema.Message, error) {
	var reply struct {
		Content string                `json:"content"`
		Calls   []schema.FunctionCall `json:"tool_calls"`
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if len(text) > 1024*1024 || decoder.Decode(&reply) != nil {
		return nil, &workflow.Failure{Kind: "output", Cause: errors.New("Codex 模型返回无效 JSON 或超过 1 MiB")}
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || len(reply.Calls) > 1 || len(reply.Calls) == 0 && strings.TrimSpace(reply.Content) == "" {
		return nil, errors.New("Codex 模型回复为空、多余内容或请求多个工具")
	}
	message := &schema.Message{Role: schema.Assistant, Content: reply.Content}
	for _, call := range reply.Calls {
		known := false
		for _, tool := range tools {
			known = known || tool != nil && tool.Name == call.Name
		}
		var args map[string]json.RawMessage
		if !known || json.Unmarshal([]byte(call.Arguments), &args) != nil || args == nil {
			return nil, errors.New("Codex 请求未注册工具或参数不是 JSON 对象")
		}
		message.ToolCalls = append(message.ToolCalls, schema.ToolCall{ID: uuid.NewString(), Type: "function", Function: call})
	}
	return message, nil
}

const codexResponseSchema = `{"type":"object","properties":{"content":{"type":"string"},"tool_calls":{"type":"array","maxItems":1,"items":{"type":"object","properties":{"name":{"type":"string"},"arguments":{"type":"string"}},"required":["name","arguments"],"additionalProperties":false}}},"required":["content","tool_calls"],"additionalProperties":false}`
