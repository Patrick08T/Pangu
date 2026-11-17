package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

// Missing rows/storage errors are not the same as an existing unpublished intent.
func evolutionUnpublishedIntent(ctx context.Context, s evolution.Scope, id string, generation int) (bool, error) {
	var unpublished bool
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT publication IS NULL OR publication='null'::jsonb FROM pangu.evolution_intents WHERE id=$1 AND namespace=$2 AND project_id=$3 AND generation=$4`, id, s.Namespace, s.ProjectID, generation).Scan(&unpublished)
	})
	return unpublished, err
}
