package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestRetentionIncludesFailureBeforeWorkspaceExists(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "unprepared-worker")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	namespace, run := uuid.NewString(), uuid.NewString()
	p, err := Register(ctx, namespace, Project{Name: "准备失败", Source: filepath.Join(t.TempDir(), "missing"), Branch: "main", WorkerID: WorkerID()})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,created_at)
 VALUES($1,$2,$3,'submit','key','payload','{}',$4,now()-interval '91 days')`, uuid.NewString(), namespace, p.ID, run)
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(ctx, run, p.ID, namespace); err == nil {
		t.Fatal("不存在的源目录未失败")
	}
	items, err := RetentionCandidates(ctx, namespace)
	if err != nil || len(items) != 1 || items[0].RunID != run {
		t.Fatalf("缺少准备失败的候选: %+v %v", items, err)
	}
	if err := Expire(ctx, run, time.Now().Add(-91*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := Expire(ctx, run, time.Now().Add(-91*24*time.Hour)); err != nil {
		t.Fatalf("未准备运行重复清理失败: %v", err)
	}
	if err := Ensure(ctx, run, p.ID, namespace); !errors.Is(err, ErrExpired) {
		t.Fatalf("过期运行还能准备工作区: %v", err)
	}
	if err := PruneHistory(ctx, namespace); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pangu.submissions WHERE run_id=$1", run).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("准备失败的提交未清理: %d %v", remaining, err)
	}
	assertRetentionHistory(t, ctx, namespace, run)
}
