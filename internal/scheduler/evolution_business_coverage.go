package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

// ObservationCoverageTx holds the same acceptance locks until the caller commits
// stable. Unknown requests cannot disappear merely because they lack a result.
func (EvolutionEvidence) ObservationCoverageTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, id string, generation int) (bool, error) {
	if err := identity.Lock(ctx, tx, scope.Namespace); err != nil {
		return false, err
	}
	if err := experience.Lock(ctx, tx, scope.Namespace, scope.ProjectID); err != nil {
		return false, err
	}
	var complete bool
	err := tx.QueryRow(ctx, evolutionBusinessCoverageSQL, scope.Namespace, scope.ProjectID, id, generation).Scan(&complete)
	return complete, err
}

const evolutionBusinessCoverageSQL = `SELECT
 i.observing_at IS NOT NULL AND i.publication IS NOT NULL
 AND (coalesce(i.publication->>'flow_id','')<>'' OR coalesce(i.publication->>'agent_id','')<>'')
 AND NOT EXISTS (
 SELECT 1 FROM pangu.submissions s
 WHERE s.namespace=i.namespace AND s.project_id=i.project_id AND s.created_at>=i.observing_at
 AND s.operation IN ('submit','cron','github','entry')
 AND (
  s.purged_at IS NOT NULL OR jsonb_typeof(s.snapshot) IS DISTINCT FROM 'object'
  OR jsonb_typeof(s.snapshot->'workflow') IS DISTINCT FROM 'object'
  OR coalesce(s.snapshot#>>'{workflow,version_id}','')=''
  OR jsonb_typeof(s.snapshot#>'{workflow,nodes}') IS DISTINCT FROM 'array'
  OR s.snapshot#>'{workflow,nodes}'='[]'::jsonb
  OR s.snapshot#>>'{workflow,version_id}' IN (i.publication->>'flow_id',i.publication->>'agent_id')
  OR EXISTS (SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(s.snapshot#>'{workflow,nodes}')='array' THEN s.snapshot#>'{workflow,nodes}' ELSE '[]'::jsonb END) n
    WHERE jsonb_typeof(n) IS DISTINCT FROM 'object' OR coalesce(n->>'id','')=''
    OR coalesce(n->>'executor','')='' OR coalesce(n->>'instruction','')=''
    OR n->>'agent_version'=i.publication->>'agent_id')
 )
 AND (s.purged_at IS NOT NULL OR s.run_id IS NULL OR NOT EXISTS (
  SELECT 1 FROM pangu.evolution_observations o WHERE o.intent_id=i.id AND o.generation=i.generation AND o.run_id=s.run_id
 ))
 ) FROM pangu.evolution_intents i WHERE i.namespace=$1 AND i.project_id=$2 AND i.id=$3 AND i.generation=$4 AND i.state='observing'`
