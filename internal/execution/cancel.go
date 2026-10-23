package execution

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func RequestCancel(ctx context.Context, namespace, runID string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", runID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.run_cancellations(run_id,namespace) VALUES($1,$2)
		ON CONFLICT (run_id) DO NOTHING`, runID, namespace)
	if err != nil {
		return errors.New("无法持久保存取消请求")
	}
	return tx.Commit(ctx)
}

func CancelRequested(ctx context.Context, runID string) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	var requested bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1)", runID).Scan(&requested)
	return requested, err
}

func CancellationFlags(ctx context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT run_id::text FROM pangu.run_cancellations WHERE run_id=ANY($1::uuid[])", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func Pending(ctx context.Context, namespace string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT run_id::text FROM pangu.run_cancellations
		WHERE namespace=$1 AND reconciled_at IS NULL ORDER BY last_checked_at,requested_at LIMIT 100`, namespace)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func Reconciled(ctx context.Context, runID string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.run_cancellations SET reconciled_at=now() WHERE run_id=$1", runID)
	return err
}

func Checked(ctx context.Context, runID string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.run_cancellations SET last_checked_at=now() WHERE run_id=$1", runID)
	return err
}
