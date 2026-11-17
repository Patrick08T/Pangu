package approval

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func PendingEvents(ctx context.Context, namespace string) ([]string, error) {
	conn, err := database.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	rows, err := conn.Query(ctx, "SELECT id::text FROM pangu.approvals WHERE namespace=$1 AND signalled_at IS NULL AND (status IN ('approved','rejected','revoked','changes_requested','invalidated') OR invalidated_at IS NOT NULL) ORDER BY coalesce(invalidated_at,decided_at),id LIMIT 100", namespace)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// SignalledState acknowledges only the exact gate snapshot that was delivered.
func SignalledState(ctx context.Context, namespace string, gate *Gate) error {
	if gate == nil || gate.ID == "" || gate.ApprovalGeneration < 1 {
		return ErrInvalid
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `UPDATE pangu.approvals SET signalled_at=now()
 WHERE namespace=$1 AND id=$2 AND approval_generation=$3 AND status=$4
 AND invalidated_at IS NOT DISTINCT FROM $5::timestamptz AND decided_at IS NOT DISTINCT FROM $6::timestamptz`, namespace, gate.ID, gate.ApprovalGeneration, gate.Status, gate.InvalidatedAt, gate.DecidedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

// Signalled is retained for legacy callers; new deliveries must use SignalledState.
func Signalled(ctx context.Context, namespace, id string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.approvals SET signalled_at=now() WHERE namespace=$1 AND id=$2", namespace, id)
	return err
}
