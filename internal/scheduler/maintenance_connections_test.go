//go:build integration

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
)

func TestMaintenanceReusesIdleDatabaseConnections(t *testing.T) {
	entryDB(t)
	t.Cleanup(database.ClosePool)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	sessions := func() int64 {
		var count int64
		if err := conn.QueryRow(context.Background(), "SELECT sessions FROM pg_stat_database WHERE datname=current_database()").Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	before := sessions()
	b := maintenanceBackend(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	b.Maintain(ctx)
	// PostgreSQL 的累计统计可能延迟刷新；等待后再检查物理连接数量。
	time.Sleep(1100 * time.Millisecond)
	created := sessions() - before
	if created > 4 {
		t.Fatalf("两轮空闲巡检新建了 %d 个数据库连接，最多应为 4 个", created)
	}
	t.Logf("两轮空闲巡检新建物理连接：%d", created)
}
