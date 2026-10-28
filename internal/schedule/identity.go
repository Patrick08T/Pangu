package schedule

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

func transaction(ctx context.Context, conn *pgx.Conn, namespace string) (pgx.Tx, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func authorizeChange(ctx context.Context, project, action, owner string) error {
	actor := identity.Current(ctx)
	if actor.ID == "" {
		return nil
	}
	return identity.Authorize(ctx, actor, project, action, owner)
}
