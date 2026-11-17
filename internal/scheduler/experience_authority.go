package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

// This capability is private to one accepted anonymous request and cannot read memory.
func (r *nodeExperience) authorizeEmpty(ctx context.Context, tx pgx.Tx, scope experience.Scope) error {
	actor := identity.Current(ctx)
	if scope.Namespace != r.namespace || scope.ProjectID != r.request.ProjectID || scope.ActorID != "" || r.request.CreatedBy != "" || actor.ID != "" || actor.TokenID != "" || actor.Namespace != "" {
		return experience.ErrForbidden
	}
	if err := identity.Lock(ctx, tx, r.namespace); err != nil {
		return err
	}
	var valid bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.submissions s
 JOIN pangu.experience_watermarks w ON w.namespace=s.namespace AND w.project_id=s.project_id AND w.run_id=s.id::text AND w.generation=1
 WHERE s.namespace=$1 AND s.project_id=$2 AND s.id=$3 AND s.created_by IS NULL
 AND (s.run_id IS NULL OR s.run_id=$4) AND w.snapshot->'versions'='[]'::jsonb)`, r.namespace, r.scope.ProjectID, r.request.ID, r.runID).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return experience.ErrForbidden
	}
	return nil
}
