package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"time"
)

type tracedModel struct {
	base model.ToolCallingChatModel
	name string
}

func (m *tracedModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	base, err := m.base.WithTools(tools)
	return &tracedModel{base: base, name: m.name}, err
}

func (m *tracedModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	roles := map[schema.RoleType]int{}
	for _, msg := range messages {
		if msg != nil {
			roles[msg.Role]++
		}
	}
	input, _ := json.Marshal(roles)
	ctx, span, err := trace.Start(ctx, "model", m.name, fmt.Sprintf("messages=%d roles=%s", len(messages), input))
	if err != nil {
		return nil, err
	}
	result, err := m.base.Generate(ctx, messages, opts...)
	var output []byte
	if result != nil {
		output, _ = json.Marshal(struct {
			Content string            `json:"content"`
			Calls   []schema.ToolCall `json:"tool_calls,omitempty"`
		}{result.Content, result.ToolCalls})
		if result.Extra == nil {
			result.Extra = map[string]any{}
		}
		result.Extra["pangu_trace_span"] = trace.Parent(ctx)
	}
	return result, errors.Join(err, span.End(string(output), err))
}

func (m *tracedModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// shortcut: 当前执行器使用完整回复；需要逐 token 追踪时再包装流的终态。
	message, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

func (events *codexEvents) trace(line []byte) {
	if events.Context == nil || events.TraceErr != nil {
		return
	}
	var raw struct {
		Type, Kind, Phase, Name, Status, Input, Output, Error string
		Span                                                  string `json:"span_id"`
		Parent                                                string `json:"parent_span_id"`
		Duration                                              int64  `json:"duration_ms"`
		Truncated                                             bool
	}
	if json.Unmarshal(line, &raw) != nil || raw.Type != "pangu.trace" {
		return
	}
	if raw.Span == "" || len(raw.Span) > 128 || len(raw.Parent) > 128 || (raw.Kind != "model" && raw.Kind != "tool") ||
		(raw.Phase != "start" && raw.Phase != "end") || raw.Duration < 0 ||
		(raw.Status != "running" && raw.Status != "succeeded" && raw.Status != "failed" && raw.Status != "cancelled") {
		events.TraceErr = errors.New("无效的原生 trace 事件")
	} else {
		parent := trace.Parent(events.Context)
		if raw.Parent != "" {
			parent = events.Prefix + raw.Parent
		}
		events.TraceErr = trace.Record(events.Context, trace.Event{SpanID: events.Prefix + raw.Span, ParentID: parent, Kind: raw.Kind, Phase: raw.Phase,
			Name: raw.Name, Status: raw.Status, Input: raw.Input, Output: raw.Output, Error: raw.Error, Duration: raw.Duration, Truncated: raw.Truncated, Time: time.Now().UTC()})
	}
	if events.TraceErr != nil && events.Cancel != nil {
		events.Cancel()
	}
}
