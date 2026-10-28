package trace

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type Event struct {
	ID        int64     `json:"id"`
	RunID     string    `json:"run_id"`
	NodeID    string    `json:"node_id"`
	NodeRunID string    `json:"node_run_id"`
	Attempt   int       `json:"attempt"`
	SpanID    string    `json:"span_id"`
	ParentID  string    `json:"parent_span_id"`
	Kind      string    `json:"kind"`
	Phase     string    `json:"phase"`
	Name      string    `json:"name"`
	Time      time.Time `json:"time"`
	Duration  int64     `json:"duration_ms"`
	Status    string    `json:"status"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	Truncated bool      `json:"truncated"`
}

type Sink func(context.Context, Event) error
type sinkKey struct{}
type parentKey struct{}

func WithSink(ctx context.Context, sink Sink) context.Context {
	return context.WithValue(ctx, sinkKey{}, sink)
}

func WithRun(ctx context.Context, run, node, nodeRun string, attempt int) context.Context {
	return WithSink(ctx, func(ctx context.Context, event Event) error {
		event.RunID, event.NodeID, event.NodeRunID, event.Attempt = run, node, nodeRun, attempt
		return Append(ctx, event)
	})
}

func Parent(ctx context.Context) string {
	value, _ := ctx.Value(parentKey{}).(string)
	return value
}

func WithParent(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, parentKey{}, id)
}

func Record(ctx context.Context, event Event) error {
	sink, _ := ctx.Value(sinkKey{}).(Sink)
	if sink == nil {
		return nil
	}
	if event.Kind != "node" && event.Kind != "model" && event.Kind != "tool" {
		return errors.New("不支持的行为追踪类型")
	}
	for _, field := range []*string{&event.Name, &event.Input, &event.Output, &event.Error} {
		value, clipped := diagnostic.Text(*field, 8192)
		*field, event.Truncated = value, event.Truncated || clipped
	}
	if err := sink(ctx, event); err != nil {
		return &workflow.Failure{Kind: "trace", Cause: err}
	}
	return nil
}

type Span struct {
	ctx     context.Context
	event   Event
	started time.Time
}

func Start(ctx context.Context, kind, name, input string) (context.Context, *Span, error) {
	event := Event{SpanID: uuid.NewString(), ParentID: Parent(ctx), Kind: kind, Phase: "start", Name: name,
		Time: time.Now().UTC(), Status: "running", Input: input}
	if err := Record(ctx, event); err != nil {
		return ctx, nil, err
	}
	return context.WithValue(ctx, parentKey{}, event.SpanID), &Span{ctx: ctx, event: event, started: time.Now()}, nil
}

func (span *Span) End(output string, cause error) error {
	if span == nil {
		return nil
	}
	event := span.event
	event.Phase, event.Input, event.Output, event.Status = "end", "", output, "succeeded"
	event.Time, event.Duration = time.Now().UTC(), time.Since(span.started).Milliseconds()
	if cause != nil {
		event.Status, event.Error = "failed", cause.Error()
		if errors.Is(cause, context.Canceled) {
			event.Status = "cancelled"
		}
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(span.ctx), 5*time.Second)
	defer cancel()
	return Record(ctx, event)
}
