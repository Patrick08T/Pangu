package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type RetentionRecord struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	RunID     string    `json:"run_id"`
	WorkerID  string    `json:"worker_id"`
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

func RetentionHistory(ctx context.Context, namespace string, offset, limit int) ([]RetentionRecord, error) {
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, errors.New("分页无效")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT id::text,project_id::text,run_id::text,worker_id,status,checked_at FROM pangu.retention_records
 WHERE namespace=$1 ORDER BY checked_at DESC,id DESC OFFSET $2 LIMIT $3`, namespace, offset, limit+1)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[RetentionRecord])
	if items == nil && err == nil {
		items = []RetentionRecord{}
	}
	return items, err
}
