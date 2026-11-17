package api

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/jackc/pgx/v5"
)

// Capability receipts reuse the current governance gate, not the broader configure role.
func experienceGovernanceAllowed(ctx context.Context, scope experience.Scope) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	scope.Action = "policy_write"
	err = learning.NewStore().Authorize(ctx, tx, scope)
	if errors.Is(err, experience.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}
