package execution

import (
	"context"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestPersistentCancelAndAttemptReceipt(t *testing.T) {
	value := os.Getenv("PANGU_TEST_DATABASE_URL")
	if value == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", value)
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	run, node := uuid.NewString(), uuid.NewString()
	item := Attempt{RunID: run, NodeRunID: node, NodeID: "writer", Attempt: 1}
	if err := Begin(ctx, &item); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := RequestCancel(ctx, "test", run); err != nil {
			t.Fatal(err)
		}
	}
	if err := Begin(ctx, &Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: "queued", Attempt: 1}); err != ErrCancelled {
		t.Fatalf("取消后的排队节点仍可启动: %v", err)
	}
	active, err := Active(ctx, run)
	if err != nil || active != 1 {
		t.Fatalf("缺少执行中证据: %d %v", active, err)
	}
	item.Status, item.Message, item.Truncated = "cancelled", "token=hidden-value", true
	if err := Finish(ctx, item); err != nil {
		t.Fatal(err)
	}
	active, err = Active(ctx, run)
	if err != nil || active != 0 {
		t.Fatalf("停止后仍有活动尝试: %d %v", active, err)
	}
	items, err := Attempts(ctx, run)
	if err != nil || len(items) != 1 || items[0].FinishedAt == nil || items[0].Message == item.Message || !items[0].Truncated {
		t.Fatalf("停止与脱敏证据错误: %+v %v", items, err)
	}
}
