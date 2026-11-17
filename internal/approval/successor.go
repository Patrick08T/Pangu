package approval

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/jackc/pgx/v5"
)

func Successor(ctx context.Context, namespace, run string) (*submission.Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	var id string
	err = conn.QueryRow(ctx, `SELECT c.request_id::text FROM pangu.plan_continuations c JOIN pangu.approvals a ON a.id=c.approval_id WHERE a.namespace=$1 AND a.run_id=$2`, namespace, run).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return submission.Get(ctx, namespace, id)
}
