package workspace

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExpiryProtectsReadersAndOnlyDeletesManagedRun(t *testing.T) {
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
	p, err := Register(ctx, "retention-test", Project{Name: "保留验证", Source: source, Branch: "main", WorkerID: WorkerID()})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if err := Ensure(ctx, run, p.ID, "retention-test"); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "UPDATE pangu.workspace_runs SET created_at=now()-interval '31 days' WHERE run_id=$1", run)
	if err != nil {
		t.Fatal(err)
	}
	node := uuid.NewString()
	if _, err := conn.Exec(ctx, "INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt) VALUES($1,$2,'active',1)", run, node); err != nil {
		t.Fatal(err)
	}
	if err := Expire(ctx, run, time.Now().Add(-30*24*time.Hour)); err == nil {
		t.Fatal("清理了活跃尝试")
	}
	if _, err := conn.Exec(ctx, "DELETE FROM pangu.node_attempts WHERE run_id=$1", run); err != nil {
		t.Fatal(err)
	}
	release, err := Lease(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if err := Expire(ctx, run, time.Now().Add(-30*24*time.Hour)); !errors.Is(err, ErrLeased) {
		t.Fatalf("未保护下载租约: %v", err)
	}
	release()
	rerun := uuid.NewString()
	if err := EnsureRerun(ctx, rerun, p.ID, "retention-test", run); err != nil {
		t.Fatal(err)
	}
	if err := Expire(ctx, run, time.Now().Add(-30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRerun(ctx, rerun, p.ID, "retention-test", run); err != nil {
		t.Fatalf("已独立准备的重跑仍依赖过期源基准: %v", err)
	}
	assertExpiryResult(t, ctx, run, p.ID, source)
}

func assertExpiryResult(t *testing.T, ctx context.Context, run, project, source string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(os.Getenv("PANGU_WORKSPACE_ROOT"), run)); !os.IsNotExist(err) {
		t.Fatal("未删除托管工作区")
	}
	if _, err := os.Stat(filepath.Join(source, ".git")); err != nil {
		t.Fatal("源项目被清理")
	}
	if _, err := Lease(ctx, run); !errors.Is(err, ErrExpired) {
		t.Fatalf("过期仍可读取: %v", err)
	}
	if err := Ensure(ctx, run, project, "retention-test"); !errors.Is(err, ErrExpired) {
		t.Fatalf("过期运行重新创建了基准: %v", err)
	}
	if err := Expire(ctx, run, time.Now().Add(-30*24*time.Hour)); err != nil {
		t.Fatalf("清理重试失败: %v", err)
	}
}
