package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func poolTestURL(t testing.TB) string {
	t.Helper()
	value := os.Getenv("PANGU_TEST_DATABASE_URL")
	if value == "" {
		t.Skip("未设置独立的 PANGU_TEST_DATABASE_URL")
	}
	t.Setenv("PANGU_DATABASE_URL", value)
	ClosePool()
	t.Cleanup(ClosePool)
	return value
}

func TestPoolReusesPhysicalConnections(t *testing.T) {
	poolTestURL(t)
	var first uint32
	for i := range 20 {
		conn, err := Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		pid := conn.Conn().PgConn().PID()
		conn.Release()
		if i == 0 {
			first = pid
		} else if pid != first {
			t.Fatalf("第 %d 次获取重新创建了物理连接", i+1)
		}
	}
}

func TestPoolBoundsConnectionsAndHonorsCancellation(t *testing.T) {
	poolTestURL(t)
	for range 4 {
		conn, err := Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("连接池耗尽后未遵守调用方期限：%v", err)
	}
}

func TestPoolReloadsConfigurationWithoutBlockingBorrowers(t *testing.T) {
	value := poolTestURL(t)
	first, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	t.Setenv("PANGU_DATABASE_URL", value+"&application_name=pangu-pool-reloaded")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	second, err := Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	var name string
	if err := second.QueryRow(ctx, "SHOW application_name").Scan(&name); err != nil || name != "pangu-pool-reloaded" {
		t.Fatalf("未读取最新连接配置：%q %v", name, err)
	}
	if first.Conn().PgConn().PID() == second.Conn().PgConn().PID() {
		t.Fatal("配置变更后仍复用了旧连接")
	}
}

func TestPoolRejectsInvalidConfigurationAfterWarmup(t *testing.T) {
	poolTestURL(t)
	conn, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	conn.Release()
	for _, value := range []string{"", "postgres://user:private-secret@%zz/db", "postgres://user:private-secret@localhost/hatchet"} {
		t.Setenv("PANGU_DATABASE_URL", value)
		if _, err := Acquire(context.Background()); err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("无效配置被忽略或凭据泄漏：%v", err)
		}
	}
}

func TestPoolDiscardsUnfinishedTransactions(t *testing.T) {
	poolTestURL(t)
	ctx := context.Background()
	first, err := Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := first.Conn().PgConn().PID()
	if _, err := first.Begin(ctx); err != nil {
		first.Release()
		t.Fatal(err)
	}
	first.Release()
	second, err := Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if second.Conn().PgConn().PID() == pid {
		t.Fatal("未结束的事务被泄漏给了下一个调用方")
	}
}

func TestPoolConcurrentAcquisitionStaysBounded(t *testing.T) {
	poolTestURL(t)
	var group sync.WaitGroup
	var pids sync.Map
	for range 20 {
		group.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			conn, err := Acquire(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Release()
			pids.Store(conn.Conn().PgConn().PID(), true)
		})
	}
	group.Wait()
	count := 0
	pids.Range(func(_, _ any) bool { count++; return true })
	if count < 1 || count > 4 {
		t.Fatalf("并发获取创建了 %d 个物理连接", count)
	}
}

func TestPoolRechecksRolePrivilegesAfterWarmup(t *testing.T) {
	value := poolTestURL(t)
	config, err := pgx.ParseConfig(value)
	if err != nil {
		t.Fatal("测试连接配置无效")
	}
	if config.Host != "127.0.0.1" || config.Port != 55439 || config.Database != "pangu_test" {
		t.Skip("权限变更测试仅允许使用隔离数据库 127.0.0.1:55439/pangu_test")
	}
	role := pgx.Identifier{config.User}.Sanitize()
	config.User, config.Password = "postgres", ""
	admin, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		t.Fatal("无法连接隔离测试数据库管理账号")
	}
	defer admin.Close(context.Background())
	conn, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	conn.Release()
	if _, err := admin.Exec(context.Background(), "ALTER ROLE "+role+" CREATEDB"); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "ALTER ROLE "+role+" NOCREATEDB")
	if conn, err := Acquire(context.Background()); err == nil {
		conn.Release()
		t.Fatal("账号权限改变后，连接池仍允许使用高权限账号")
	}
}

func TestPoolReloadsServiceFile(t *testing.T) {
	value := poolTestURL(t)
	file := filepath.Join(t.TempDir(), "pg_service.conf")
	t.Setenv("PGSERVICEFILE", file)
	t.Setenv("PANGU_DATABASE_URL", value+"&service=maintenance")
	for _, want := range []string{"before_reload", "after_reload"} {
		if err := os.WriteFile(file, []byte("[maintenance]\napplication_name="+want+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		conn, err := Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var got string
		err = conn.QueryRow(context.Background(), "SHOW application_name").Scan(&got)
		conn.Release()
		if err != nil || got != want {
			t.Fatalf("连接配置文件更新未生效：得到 %q，期望 %q，错误 %v", got, want, err)
		}
	}
}

func BenchmarkMaintenanceConnections(b *testing.B) {
	poolTestURL(b)
	b.Run("新建连接", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			conn, err := Open(context.Background())
			if err != nil {
				b.Fatal(err)
			}
			conn.Close(context.Background())
		}
	})
	b.Run("复用连接", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			conn, err := Acquire(context.Background())
			if err != nil {
				b.Fatal(err)
			}
			conn.Release()
		}
	})
}
