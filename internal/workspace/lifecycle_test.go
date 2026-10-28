package workspace

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"os"
	"testing"
	"time"
)

func TestRetentionResumesIntentAndPreservesConfiguration(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "retention-worker")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := sourceRepository(t)
	namespace := uuid.NewString()
	p, err := Register(ctx, namespace, Project{Name: "中断恢复", Source: source, Branch: "main", WorkerID: WorkerID()})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if err := Ensure(ctx, run, p.ID, namespace); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "UPDATE pangu.workspace_runs SET created_at=now()-interval '91 days',expired_at=now() WHERE run_id=$1", run)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := RetentionCandidates(ctx, namespace)
	if err != nil || len(candidates) != 1 || candidates[0].ExpiredAt == nil {
		t.Fatalf("中断意图丢失: %+v %v", candidates, err)
	}
	if err := Expire(ctx, run, time.Now().Add(-91*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := PruneHistory(ctx, namespace); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pangu.workspace_runs WHERE run_id=$1", run).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("90天运行历史没有清理: %d %v", remaining, err)
	}
	assertRetentionHistory(t, ctx, namespace, run)
	assertActiveFireProtected(t, ctx, namespace, p.ID)
	if _, err := ProjectByID(ctx, namespace, p.ID); err != nil {
		t.Fatal("清理删除了有效项目配置")
	}
	candidates, err = RetentionCandidates(ctx, namespace)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("已完成清理仍被选中: %+v %v", candidates, err)
	}
}

func assertRetentionHistory(t *testing.T, ctx context.Context, namespace, run string) {
	t.Helper()
	items, err := RetentionHistory(ctx, namespace, 0, 20)
	if err != nil || len(items) != 1 || items[0].RunID != run || items[0].Status != "purged" {
		t.Fatalf("运行删除后缺少独立清理记录: %+v %v", items, err)
	}
	other, err := RetentionHistory(ctx, "other-"+namespace, 0, 20)
	if err != nil || len(other) != 0 {
		t.Fatalf("泄露了其他命名空间清理记录: %+v %v", other, err)
	}
	if expired, err := Expired(ctx, run); err != nil || !expired {
		t.Fatalf("删除历史后重新展示过期数据: %v %v", expired, err)
	}
}

func assertActiveFireProtected(t *testing.T, ctx context.Context, namespace, project string) {
	t.Helper()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	id := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.schedules(id,namespace,project_id,name,expression,workflow,active_slot) VALUES($1,$2,$3,'annual','0 0 1 1 *','{}',now()-interval '91 days')`, id, namespace, project)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.schedule_fires(schedule_id,slot,snapshot,status,created_at) SELECT id,active_slot,'{}','accepted',active_slot FROM pangu.schedules WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := PruneHistory(ctx, namespace); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pangu.schedule_fires WHERE schedule_id=$1", id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("删除了仍被计划引用的触发记录: %d %v", count, err)
	}
}
