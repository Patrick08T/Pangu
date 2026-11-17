package trace

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestLoopTraceFiltersExactNodeExecution(t *testing.T) {
	id := uuid.NewString()
	query, err := ParseQuery(url.Values{"node_run_id": {id}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseQuery(url.Values{"node_run_id": {"invalid"}}); err == nil {
		t.Fatal("接受了无效节点运行 ID")
	}
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err = database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	for _, node := range []string{id, uuid.NewString()} {
		_, span, err := Start(WithRun(ctx, run, "qa", node, 1), "tool", "验收", "")
		if err != nil {
			t.Fatal(err)
		}
		if err = span.End("证据", nil); err != nil {
			t.Fatal(err)
		}
	}
	page, err := List(ctx, run, query)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("%v %+v", err, page)
	}
	for _, event := range page.Items {
		if event.NodeRunID != id {
			t.Fatal("混入其他轮次日志")
		}
	}
}
