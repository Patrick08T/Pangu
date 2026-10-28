package approval

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func PendingEvents(ctx context.Context, namespace string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT id::text FROM pangu.approvals WHERE namespace=$1 AND signalled_at IS NULL AND status IN ('approved','rejected','revoked') AND expires_at>now() ORDER BY decided_at,id LIMIT 100", namespace)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func Signalled(ctx context.Context, namespace, id string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.approvals SET signalled_at=now() WHERE namespace=$1 AND id=$2", namespace, id)
	return err
}
