package trace

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestTracePaginationAndPurgePreventsLateWrites(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	ctx = WithRun(ctx, run, "writer", uuid.NewString(), 1)
	for range 3 {
		_, span, err := Start(ctx, "tool", "read", "token=secret-value")
		if err != nil {
			t.Fatal(err)
		}
		if err = span.End("完成", nil); err != nil {
			t.Fatal(err)
		}
	}
	page, err := List(ctx, run, Query{Limit: 2, Kind: "tool"})
	if err != nil || len(page.Items) != 2 || page.NextBefore == nil {
		t.Fatalf("分页失败: %+v %v", page, err)
	}
	next, err := List(ctx, run, Query{Limit: 100, Before: *page.NextBefore})
	if err != nil || len(next.Items) != 4 || next.Items[0].ID >= *page.NextBefore {
		t.Fatalf("游标重复/丢失: %+v %v", next, err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "SELECT pangu.purge_run_payloads($1)", run); err != nil {
		t.Fatal(err)
	}
	page, err = List(ctx, run, Query{Limit: 100})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("追踪未清理: %+v %v", page, err)
	}
	if _, _, err = Start(ctx, "tool", "late", ""); err == nil {
		t.Fatal("晚到事件恢复了已清理正文")
	}
}
