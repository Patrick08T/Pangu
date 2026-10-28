package agent

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"strings"
	"testing"
)

func TestNativeTraceScopesRedactsAndFailsClosed(t *testing.T) {
	var items []trace.Event
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = trace.WithSink(ctx, func(_ context.Context, e trace.Event) error { items = append(items, e); return nil })
	events := &codexEvents{Context: ctx, Cancel: cancel, Prefix: "session/"}
	_, err := events.Write([]byte(`{"type":"pangu.trace","span_id":"2","parent_span_id":"1","kind":"tool","phase":"start","name":"exec_command","status":"running","input":"token=private-secret","run_id":"forged"}` + "\n"))
	if err != nil || len(items) != 1 || items[0].SpanID != "session/2" || items[0].ParentID != "session/1" || items[0].RunID != "" || strings.Contains(items[0].Input, "private-secret") {
		t.Fatalf("原生 trace 未隔离: %+v %v", items, err)
	}
	events.Context = trace.WithSink(ctx, func(context.Context, trace.Event) error { return errors.New("持久化失败") })
	_, err = events.Write([]byte(`{"type":"pangu.trace","span_id":"2","kind":"tool","phase":"end","name":"exec_command","status":"succeeded"}` + "\n"))
	if err == nil || ctx.Err() == nil {
		t.Fatal("追踪丢失后仍允许执行")
	}
}

func TestTraceFailureStopsCodexWithoutCallingItUserCancellation(t *testing.T) {
	setupCodex(t)
	expected := errors.New("追踪写入失败")
	ctx := trace.WithSink(context.Background(), func(context.Context, trace.Event) error { return expected })
	_, err := Execute(ctx, codexInput("trace-failure"))
	if !errors.Is(err, expected) || workflow.FailureKind(err) != "trace" || workflow.CanRetry(err) {
		t.Fatalf("追踪失败被误分类: %v", err)
	}
}
