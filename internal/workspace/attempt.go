package workspace

import (
	"context"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
)

func failAttempt(node string, attempt int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := database.Open(ctx)
	if err != nil {
		return
	}
	defer conn.Close(context.Background())
	// 已提交但回执丢失时，不能把数据库中已发布的检查点改为失败。
	_, _ = conn.Exec(ctx, "UPDATE pangu.workspace_attempts SET status='failed' WHERE node_run_id=$1 AND attempt=$2 AND status<>'published'", node, attempt)
}
