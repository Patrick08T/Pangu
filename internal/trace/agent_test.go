package trace

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestSpanRecordsRealOutcomeAndRedactsBeforeSink(t *testing.T) {
	var events []Event
	ctx := WithSink(context.Background(), func(_ context.Context, event Event) error {
		events = append(events, event)
		return nil
	})
	ctx, span, err := Start(ctx, "tool", "read_file", `api_key=private-value-123`)
	if err != nil {
		t.Fatal(err)
	}
	_, child, err := Start(ctx, "model", "test", "请求摘要")
	if err != nil {
		t.Fatal(err)
	}
	if err = child.End("", errors.New("token=private-token-456")); err != nil {
		t.Fatal(err)
	}
	if err = span.End("实际结果", nil); err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[1].ParentID != events[0].SpanID || events[2].Status != "failed" || events[3].Output != "实际结果" {
		t.Fatalf("追踪链或结果错误：%+v", events)
	}
	for _, event := range events {
		if strings.Contains(event.Input+event.Output+event.Error, "private-") {
			t.Fatalf("凭据到达写入端：%+v", event)
		}
	}
}

func TestTraceQueryRejectsAmbiguousOrUnboundedInput(t *testing.T) {
	for _, raw := range []string{"limit=0", "limit=101", "after=-1", "kind=reasoning", "after=1&after=2", "unknown=x"} {
		values, _ := url.ParseQuery(raw)
		if _, err := ParseQuery(values); err == nil {
			t.Fatalf("接受无效查询 %s", raw)
		}
	}
	query, err := ParseQuery(url.Values{"node": {"review"}, "kind": {"tool"}, "after": {"10"}})
	if err != nil || query.NodeID != "review" || query.After != 10 || query.Limit != 50 {
		t.Fatalf("查询错误：%+v %v", query, err)
	}
}

func TestFailedTraceWriteIsReturned(t *testing.T) {
	expected := errors.New("追踪存储不可用")
	ctx := WithSink(context.Background(), func(context.Context, Event) error { return expected })
	if _, _, err := Start(ctx, "model", "test", ""); !errors.Is(err, expected) || workflow.CanRetry(err) || workflow.FailureKind(err) != "trace" {
		t.Fatalf("追踪失败未反馈：%v", err)
	}
}
