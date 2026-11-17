package execution

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

// ModelRetryAt 从已确认停止的最终尝试计算时间；重放时不会重新开始计时。
func ModelRetryAt(ctx context.Context, run, node string, sequence int) (*time.Time, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	var status, category string
	var finished *time.Time
	err = conn.QueryRow(ctx, `SELECT status,category,finished_at FROM pangu.node_attempts
		WHERE run_id=$1 AND node_run_id=$2 ORDER BY attempt DESC LIMIT 1`, run, node).Scan(&status, &category, &finished)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if status != "failed" || finished == nil || !workflow.ModelRetryCategory(category) {
		return nil, nil
	}
	at := finished.Add(workflow.ModelRetryDelay(sequence))
	return &at, nil
}
