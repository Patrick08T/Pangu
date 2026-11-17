package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

// The same locked proof protects dispatch and post-RPC confirmation. Compensation
// deliberately does not depend on the source that made the candidate ineligible.
func evolutionPublishedForwardTx(ctx context.Context, tx pgx.Tx, namespace, key string, expected evolution.EntrySnapshot) error {
	var id, direction string
	var generation int
	err := tx.QueryRow(ctx, `SELECT i.id::text,i.generation,sw.direction FROM pangu.evolution_intents i JOIN pangu.evolution_switches sw ON sw.intent_id=i.id AND sw.generation=i.generation WHERE i.namespace=$1 AND i.project_id=$2 AND i.id::text||':'||i.generation::text||':'||sw.entry_key||':'||sw.direction=$3 AND sw.result=$4`, namespace, expected.ProjectID, key, evolutionJSON(expected)).Scan(&id, &generation, &direction)
	if errors.Is(err, pgx.ErrNoRows) {
		return evolution.ErrConflict
	}
	if err != nil {
		return err
	}
	if direction == "rollback" {
		return nil
	}
	scope := evolution.Scope{Namespace: namespace, ProjectID: expected.ProjectID}
	return newEvolutionService().PublishedEvidenceTx(ctx, tx, scope, id, generation)
}
