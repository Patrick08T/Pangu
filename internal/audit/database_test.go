package audit

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestAuditIntentAndOutcomePersistWithoutSensitiveBody(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ns := uuid.NewString()
	event, err := Begin(ctx, Entry{Namespace: ns, ActorID: uuid.NewString(), RequestID: uuid.NewString(), Action: "POST /catalog/{id}/publish", ObjectID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	items, err := List(ctx, ns, "", 0)
	if err != nil || len(items) != 1 || items[0].Outcome != "pending" {
		t.Fatalf("意图未持久化: %+v %v", items, err)
	}
	event.ProjectID = uuid.NewString()
	event.ResultID = uuid.NewString()
	if err = Finish(ctx, event, 201); err != nil {
		t.Fatal(err)
	}
	items, err = List(ctx, ns, event.ProjectID, 0)
	if err != nil || len(items) != 1 || items[0].Outcome != "succeeded" || items[0].ResultID != event.ResultID {
		t.Fatalf("结果未持久化: %+v %v", items, err)
	}
	if err = Finish(ctx, event, 500); err == nil {
		t.Fatal("审计终态被改写")
	}
	items, err = List(ctx, uuid.NewString(), "", 0)
	if err != nil || len(items) != 0 {
		t.Fatal("跨命名空间泄漏")
	}
}
