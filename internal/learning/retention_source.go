package learning

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func eraseWithdrawnSourceTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, sourceID string) error {
	for {
		rows, err := tx.Query(ctx, `SELECT v.id FROM pangu.experience_versions v LEFT JOIN pangu.experience_states s ON s.version_id=v.id
   WHERE v.namespace=$1 AND v.project_id=$2 AND `+retentionSourceIDSQL()+`
   AND (NOT s.tombstoned OR EXISTS(SELECT 1 FROM pangu.experience_bodies b WHERE b.version_id=v.id)) ORDER BY v.id LIMIT 1000`, scope.Namespace, scope.ProjectID, sourceID)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return eraseSourceCheckersTx(ctx, tx, scope, sourceID)
		}
		if _, err = experience.EraseVersionsTx(ctx, tx, scope.Namespace, scope.ProjectID, ids, "source_withdrawn"); err != nil {
			return err
		}
	}
}

func eraseSourceCheckersTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, sourceID string) error {
	_, err := tx.Exec(ctx, `DELETE FROM pangu.learning_checker_bodies b USING pangu.learning_checkers c
  WHERE b.checker_id=c.id AND c.namespace=$1 AND c.project_id=$2 AND EXISTS(
   SELECT 1 FROM pangu.experience_versions v WHERE v.namespace=$1 AND v.project_id=$2
   AND `+retentionSourceIDSQL()+` AND (c.content_hash=v.body_hash OR EXISTS(
    SELECT 1 FROM pangu.learning_claim_requests r WHERE r.namespace=$1 AND r.project_id=$2 AND r.version_id=v.id AND r.checker_id=c.id)))`, scope.Namespace, scope.ProjectID, sourceID)
	return err
}
