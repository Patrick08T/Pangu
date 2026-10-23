package submission

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func Dispatch(ctx context.Context, namespace, id string, send func(context.Context, *Record) (string, error)) (*Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", "submission:"+id); err != nil {
		return nil, err
	}
	item, _, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND id=$2", namespace, id))
	if err != nil || item.RunID != "" {
		return item, err
	}
	if time.Since(item.CreatedAt) > 89*24*time.Hour {
		return item, ErrExpired
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.submissions SET last_checked_at=now() WHERE id=$1", id); err != nil {
		return item, err
	}
	run, sendErr := send(ctx, item)
	if sendErr != nil {
		message, _ := diagnostic.Text(sendErr.Error(), 4096)
		_, _ = conn.Exec(ctx, "UPDATE pangu.submissions SET last_error=$2 WHERE id=$1", id, message)
		return item, sendErr
	}
	runID, err := uuid.Parse(run)
	if err != nil {
		return item, errors.New("调度服务没有返回有效运行 ID")
	}
	run = runID.String()
	if err = bindReceipt(ctx, conn, id, run); err != nil {
		return item, err
	}
	item.RunID, item.Status = run, "accepted"
	return item, nil
}

func bindReceipt(ctx context.Context, conn *pgx.Conn, id, run string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	// 与清理互斥，避免迟到回执在清理事务之后重新关联完整正文。
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(hashtextextended($1,0))", "retention:"+run); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.submissions SET run_id=$2,last_error='' WHERE id=$1", id, run); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT pangu.purge_run_payloads($1) WHERE EXISTS(SELECT 1 FROM pangu.workspace_runs WHERE run_id=$1 AND expired_at IS NOT NULL)
 OR EXISTS(SELECT 1 FROM pangu.retention_records WHERE run_id=$1 AND status='purged')`, run)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func Pending(ctx context.Context, namespace string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT id::text FROM pangu.submissions WHERE namespace=$1 AND run_id IS NULL
		AND created_at>now()-interval '89 days' ORDER BY last_checked_at,created_at LIMIT 100`, namespace)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
