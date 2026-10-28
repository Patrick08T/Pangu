package submission

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

// SaveHistory 只接收调度层构造的终态元数据，不保存输入、输出或日志。
func SaveHistory(ctx context.Context, namespace, id string, finished time.Time, summary []byte) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary)
 SELECT run_id,namespace,$3,$4 FROM pangu.submissions WHERE namespace=$1 AND run_id=$2
 ON CONFLICT(run_id) DO NOTHING`, namespace, id, finished, summary)
	return err
}

func History(ctx context.Context, namespace, id string) ([]byte, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	var body []byte
	err = conn.QueryRow(ctx, `SELECT summary FROM pangu.run_history WHERE namespace=$1 AND run_id=$2`, namespace, id).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return body, err
}

// HistoryPending 按上次检查轮转，活跃或错误记录不能饿死其他运行。
func HistoryPending(ctx context.Context, namespace string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `WITH pending AS (
 SELECT s.id FROM pangu.submissions s WHERE s.namespace=$1 AND s.run_id IS NOT NULL
 AND NOT EXISTS(SELECT 1 FROM pangu.run_history h WHERE h.run_id=s.run_id)
 ORDER BY s.history_checked_at,s.created_at LIMIT 100 FOR UPDATE SKIP LOCKED)
 UPDATE pangu.submissions s SET history_checked_at=now() FROM pending p WHERE s.id=p.id RETURNING s.run_id::text`, namespace)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func RunIDs(ctx context.Context, namespace, project string, since, until time.Time, offset, limit int64) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT run_id::text FROM pangu.submissions WHERE namespace=$1
 AND ($2='' OR project_id::text=$2) AND run_id IS NOT NULL AND created_at>=$3 AND created_at<=$4
 ORDER BY created_at DESC,run_id DESC OFFSET $5 LIMIT $6`, namespace, project, since, until, offset, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
