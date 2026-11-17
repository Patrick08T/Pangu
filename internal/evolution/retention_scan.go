package evolution

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RetentionScanTarget 用于服务器维护，不授予交互用户权限。
type RetentionScanTarget struct {
	Scope         Scope
	ID            string
	Generation    int
	FrozenActorID string
}

// ListRetentionTargets 服务器keyset扫描；此快照不授权擦除，逐项仍须ReconcileRetention复核。
func ListRetentionTargets(ctx context.Context, namespace, after string, limit int) ([]RetentionScanTarget, error) {
	if strings.TrimSpace(namespace) == "" || limit < 1 || limit > 1000 {
		return nil, ErrInvalid
	}
	if after != "" {
		if _, err := uuid.Parse(after); err != nil {
			return nil, ErrInvalid
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := database.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	rows, err := conn.Query(ctx, retentionScanSQL, namespace, after, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (RetentionScanTarget, error) {
		return retentionScanRow(row, namespace)
	})
}

func retentionScanRow(row pgx.Row, namespace string) (RetentionScanTarget, error) {
	var target RetentionScanTarget
	var policy experience.Policy
	var raw []byte
	var revision int64
	target.Scope.Namespace = namespace
	err := row.Scan(&target.ID, &target.Scope.ProjectID, &target.Generation, &target.FrozenActorID, &revision, &raw)
	if err != nil {
		return target, err
	}
	if target.FrozenActorID == "" || len(raw) == 0 {
		return target, ErrForbidden
	}
	if err = json.Unmarshal(raw, &policy); err != nil {
		return target, err
	}
	if policy.ProjectID != target.Scope.ProjectID || policy.ActorID != target.FrozenActorID || policy.Revision != revision {
		return target, ErrForbidden
	}
	return target, nil
}

const retentionScanSQL = `SELECT i.id::text,i.project_id::text,i.generation,coalesce(p.actor_id::text,''),i.policy_revision,p.policy
 FROM pangu.evolution_intents i LEFT JOIN pangu.experience_policies p
 ON p.namespace=i.namespace AND p.project_id=i.project_id AND p.revision=i.policy_revision
 WHERE i.namespace=$1 AND ($2='' OR i.id>NULLIF($2,'')::uuid) AND NOT (
 coalesce(jsonb_typeof(i.request#>'{_retention,erased_at}')='string',false)
 AND coalesce(i.request#>>'{_retention,erased_at}','')<>''
 AND coalesce(i.request#>'{request,payload,agent}'='{}'::jsonb,false)
 AND NOT coalesce((i.request#>>'{_retention,awaiting_stop}')::bool,false)
 AND NOT EXISTS(SELECT 1 FROM pangu.catalog_experiment_candidates c WHERE c.experiment_id=i.experiment_id AND c.payload IS NOT NULL)
 AND NOT EXISTS(SELECT 1 FROM pangu.catalog_versions v JOIN pangu.catalog_version_bodies b ON b.version_id=v.id
 WHERE v.source_experiment=i.experiment_id AND v.project_id=i.project_id AND b.spec IS NOT NULL))
 ORDER BY i.id LIMIT $3`
