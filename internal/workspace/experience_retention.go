package workspace

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// PurgeExperienceRunTx clears a stopped, workspace-free evaluator in the caller's
// authorized retention transaction. The caller must hold evolution-case:<request>
// and establish the authoritative cutoff; this function does not grant retention consent.
func PurgeExperienceRunTx(ctx context.Context, tx pgx.Tx, namespace, project, request, run string) error {
	var valid bool
	err := tx.QueryRow(ctx, `SELECT s.operation='evolution' AND s.snapshot->>'purpose'='evaluation'
	 AND NOT EXISTS(SELECT 1 FROM pangu.workspace_runs w WHERE w.run_id=s.run_id)
	 FROM pangu.submissions s WHERE s.namespace=$1 AND s.project_id=$2 AND s.id=$3 AND s.run_id=$4
	 FOR UPDATE OF s`, namespace, project, request, run).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("experience retention requires a workspace-free evaluation submission")
	}
	if err := purgePayloads(ctx, tx, run); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.node_attempts SET message='',truncated=true,
	 evidence='[]'::jsonb,evidence_truncated=true WHERE run_id=$1`, run)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `WITH done AS (UPDATE pangu.submissions SET purged_at=clock_timestamp(),
	 retention_checked_at=clock_timestamp() WHERE id=$1 AND purged_at IS NULL RETURNING *)
	 INSERT INTO pangu.retention_records(namespace,project_id,run_id,worker_id,status)
	 SELECT d.namespace,d.project_id,d.run_id,p.worker_id,'purged' FROM done d
	 JOIN pangu.projects p ON p.id=d.project_id AND p.namespace=d.namespace`, request)
	return err
}
