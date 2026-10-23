package audit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestAuditReconcileContinuesPastInvalidFilesAndFirstPage(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_AUDIT_SPOOL_ROOT", t.TempDir())
	e := Entry{ID: uuid.NewString(), Namespace: uuid.NewString(), ActorID: uuid.NewString(), RequestID: uuid.NewString(), Action: "POST /runs/{id}/cancel", CreatedAt: time.Now()}
	root, err := spoolRoot(e.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for i := range 210 {
		f, err := root.Create(fmt.Sprintf(".pending-%03d", i))
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	f, err := root.Create("000-invalid.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("invalid JSON")
	_ = f.Close()
	if err := Queue(e, 202); err != nil {
		t.Fatal(err)
	}
	if n, err := Reconcile(context.Background(), e.Namespace); n != 1 || err == nil {
		t.Fatalf("坏文件或前 200 项阻塞补录: n=%d err=%v", n, err)
	}
	if _, err := root.Stat("000-invalid.json"); err != nil {
		t.Fatal("错误文件未保留供核对", err)
	}
}

func TestAuditSpoolReplaysAfterDatabaseRecoveryWithoutChangingTerminal(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_AUDIT_SPOOL_ROOT", t.TempDir())
	t.Setenv("PANGU_DATABASE_URL", "invalid://unavailable")
	ctx := context.Background()
	e := Entry{ID: uuid.NewString(), Namespace: uuid.NewString(), ActorID: uuid.NewString(), RequestID: uuid.NewString(), Action: "POST /runs/{id}/cancel", ObjectID: uuid.NewString(), CreatedAt: time.Now().Add(-2 * time.Minute)}
	if err := Queue(e, 0); err != nil {
		t.Fatal(err)
	}
	if err := Queue(e, 202); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(ctx, e.Namespace); err == nil {
		t.Fatal("数据库不可用却删除了补录记录")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := Reconcile(ctx, e.Namespace); err != nil || count != 2 {
		t.Fatalf("补录失败: %d %v", count, err)
	}
	items, err := List(ctx, e.Namespace, "", 0)
	if err != nil || len(items) != 1 || items[0].Outcome != "succeeded" || items[0].ObjectID != e.ObjectID {
		t.Fatalf("审计结果错误: %+v %v", items, err)
	}
	if count, err := Reconcile(ctx, e.Namespace); err != nil || count != 0 {
		t.Fatalf("重复补录不幂等: %d %v", count, err)
	}
	if err := Queue(e, 500); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(ctx, e.Namespace); err == nil {
		t.Fatal("冲突结果被静默丢弃")
	}
	items, err = List(ctx, e.Namespace, "", 0)
	if err != nil || items[0].Outcome != "succeeded" {
		t.Fatal("补录覆盖了原始终态")
	}
}

func TestAuditSpoolUnknownIntentRemainsUnknown(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_AUDIT_SPOOL_ROOT", t.TempDir())
	e := Entry{ID: uuid.NewString(), Namespace: uuid.NewString(), ActorID: uuid.NewString(), RequestID: uuid.NewString(), Action: "POST /runs/{id}/cancel", CreatedAt: time.Now().Add(-2 * time.Minute)}
	if err := Queue(e, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(context.Background(), e.Namespace); err != nil {
		t.Fatal(err)
	}
	items, err := List(context.Background(), e.Namespace, "", 0)
	if err != nil || len(items) != 1 || items[0].Outcome != "unknown" {
		t.Fatalf("崩溃后意图被猜测成成功: %+v %v", items, err)
	}
}
