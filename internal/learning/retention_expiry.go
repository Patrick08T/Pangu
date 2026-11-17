package learning

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// The full source identity matters: expiry of an old hash must not erase a new
// consented revision. Missing consent is not proof of expiry or deletion authority.
var retentionConsentSQL = `EXISTS(SELECT 1 FROM jsonb_array_elements(v.sources) src
 JOIN pangu.learning_source_consents c ON c.namespace=v.namespace AND c.project_id=v.project_id
 AND c.source_id=` + retentionUUIDSQL("src->>'id'") + ` AND c.source_kind=src->>'kind' AND c.source_hash=src->>'hash'
 AND c.run_id=` + retentionUUIDSQL("src->>'run_id'") + ` AND c.node_id=src->>'node_id'
 WHERE ` + retentionUUIDSQL("src->>'project_id'") + `=v.project_id AND `
var expiredConsentSQL = retentionConsentSQL + `(c.revoked_at IS NOT NULL OR c.expires_at<=clock_timestamp()))`
var revokedConsentSQL = retentionConsentSQL + `c.revoked_at IS NOT NULL)`

type retentionTarget struct{ ID, Reason string }

func retentionTargetsTx(ctx context.Context, tx pgx.Tx, namespace, project string, limit int) ([]retentionTarget, error) {
	rows, err := tx.Query(ctx, `SELECT v.id,CASE WHEN `+revokedConsentSQL+` THEN 'source_withdrawn'
  WHEN v.expires_at<=clock_timestamp() THEN 'retention_expired' ELSE 'source_expired' END
  FROM pangu.experience_versions v LEFT JOIN pangu.experience_states s ON s.version_id=v.id
  WHERE v.namespace=$1 AND v.project_id=$2 AND (NOT s.tombstoned OR EXISTS(
   SELECT 1 FROM pangu.experience_bodies b WHERE b.version_id=v.id))
  AND (v.expires_at<=clock_timestamp() OR `+expiredConsentSQL+`) ORDER BY v.id LIMIT $3`, namespace, project, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[retentionTarget])
}

func purgeCheckerBodiesTx(ctx context.Context, tx pgx.Tx, namespace, project string, limit int) (int64, error) {
	tag, err := tx.Exec(ctx, `DELETE FROM pangu.learning_checker_bodies b WHERE b.checker_id IN (
  SELECT c.id FROM pangu.learning_checkers c JOIN pangu.learning_checker_bodies present ON present.checker_id=c.id
  WHERE c.namespace=$1 AND c.project_id=$2 AND (c.expires_at<=clock_timestamp() OR EXISTS(
   SELECT 1 FROM pangu.experience_versions v WHERE v.namespace=$1 AND v.project_id=$2 AND `+expiredConsentSQL+`
   AND (v.body_hash=c.content_hash OR EXISTS(SELECT 1 FROM pangu.learning_claim_requests r
    WHERE r.namespace=$1 AND r.project_id=$2 AND r.version_id=v.id AND r.checker_id=c.id))))
  ORDER BY c.id LIMIT $3)`, namespace, project, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
