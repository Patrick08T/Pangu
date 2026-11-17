package evaluation

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func experimentTransaction[T any](ctx context.Context, work func(pgx.Tx) (T, error)) (T, error) {
	var zero T
	conn, err := database.Open(ctx)
	if err != nil {
		return zero, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(context.Background())
	out, err := work(tx)
	if err != nil {
		return zero, err
	}
	if err = tx.Commit(ctx); err != nil {
		return zero, err
	}
	return out, nil
}

func committedReservation(ctx context.Context, namespace string, e *FrozenExperiment) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	var exists bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evaluation_experiments WHERE namespace=$1 AND id=$2 AND generation=$3 AND spec_hash=$4 AND status='active')`, namespace, e.ID, e.Generation, e.Hash).Scan(&exists)
	return exists, err
}

func experimentDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func validExperimentIDs(ids ...string) bool {
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return false
		}
	}
	return true
}
