package scheduler

import (
	"context"
	"errors"
	"slices"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

var ErrEvolutionConsumerStopUnknown = errors.New("进化正文消费者停止状态尚未确认")

type evolutionConsumer struct {
	request, run, actor string
	evaluation          bool
}

// EvolutionRetentionGuard is a trusted, same-transaction SQL guard, not an API
// assertion. Unknown evidence is an error; false only denotes a known blocker.
func EvolutionRetentionGuard(ctx context.Context, tx pgx.Tx, scope evolution.Scope, target evolution.RetentionTarget, phase string) (bool, error) {
	if phase != "stopped" && phase != "unreferenced" {
		return false, evolution.ErrInvalid
	}
	if err := evolutionConsumerTargetTx(ctx, tx, scope, target); err != nil {
		return false, err
	}
	versions, err := evolutionConsumerVersionsTx(ctx, tx, scope, target.VersionIDs)
	if err != nil {
		return false, err
	}
	if phase == "unreferenced" {
		var clear bool
		err = tx.QueryRow(ctx, evolutionConsumersUnreferencedSQL, scope.Namespace, scope.ProjectID, versions, target.VersionIDs).Scan(&clear)
		return clear, err
	}
	if err = evolutionConsumerLineageTx(ctx, tx, scope, versions); err != nil {
		return false, err
	}
	consumers, err := evolutionConsumersTx(ctx, tx, scope, target, versions)
	if err != nil {
		return false, err
	}
	stopped := true
	for _, item := range consumers {
		if item.run == "" {
			return false, ErrEvolutionConsumerStopUnknown
		}
		closure, err := evolutionConsumerClosureTx(ctx, tx, scope, item)
		if err != nil {
			return false, err
		}
		for _, child := range closure {
			ok, err := evolutionConsumerStoppedTx(ctx, tx, scope, child)
			if err != nil {
				return false, err
			}
			stopped = stopped && ok
		}
	}
	return stopped, nil
}

func evolutionConsumerTargetTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, target evolution.RetentionTarget) error {
	var experiment string
	var versions []string
	err := tx.QueryRow(ctx, `SELECT coalesce(experiment_id::text,''),array_remove(ARRAY[nullif(publication->>'agent_id',''),nullif(publication->>'flow_id','')],NULL)
 FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2 AND id=$3 AND generation=$4`, scope.Namespace, scope.ProjectID, target.IntentID, target.Generation).Scan(&experiment, &versions)
	if err != nil {
		return err
	}
	actual := slices.Clone(target.VersionIDs)
	slices.Sort(actual)
	slices.Sort(versions)
	if experiment != target.ExperimentID || !slices.Equal(actual, versions) {
		return evolution.ErrConflict
	}
	var valid int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 JOIN pangu.catalog_version_state s ON s.version_id=v.id WHERE o.namespace=$1 AND v.project_id=$2
 AND v.id=ANY($3::uuid[]) AND v.source_kind='auto' AND s.state='revoked'`, scope.Namespace, scope.ProjectID, target.VersionIDs).Scan(&valid)
	if err == nil && valid != len(target.VersionIDs) {
		return evolution.ErrConflict
	}
	return err
}

func evolutionConsumerVersionsTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, roots []string) ([]string, error) {
	rows, err := tx.Query(ctx, evolutionConsumerVersionsSQL, scope.Namespace, scope.ProjectID, roots)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func evolutionConsumersTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, target evolution.RetentionTarget, versions []string) ([]evolutionConsumer, error) {
	rows, err := tx.Query(ctx, evolutionConsumersSQL, scope.Namespace, scope.ProjectID, versions, target.IntentID, target.Generation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []evolutionConsumer
	for rows.Next() {
		var item evolutionConsumer
		if err = rows.Scan(&item.request, &item.run, &item.evaluation, &item.actor); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func evolutionConsumerLineageTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, versions []string) error {
	if len(versions) == 0 {
		return nil
	}
	var unknown bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.submissions s WHERE s.namespace=$1 AND s.project_id=$2
 AND (s.purged_at IS NOT NULL OR EXISTS(SELECT 1 FROM pangu.run_debug d WHERE d.run_id=s.run_id AND d.expired))
 AND NOT coalesce(jsonb_typeof(s.snapshot#>'{workflow,nodes}')='array',false)
 AND NOT EXISTS(SELECT 1 FROM pangu.evolution_executions e WHERE e.request_id=s.id)
 AND NOT coalesce(s.snapshot#>>'{workflow,version_id}'=ANY($3::text[]),false))`, scope.Namespace, scope.ProjectID, versions).Scan(&unknown)
	if err == nil && unknown {
		return ErrEvolutionConsumerStopUnknown
	}
	return err
}

func evolutionConsumerStoppedTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, item evolutionConsumer) (bool, error) {
	if item.run == "" {
		return false, ErrEvolutionConsumerStopUnknown
	}
	if item.evaluation {
		return evolutionConsumerEvaluationTx(ctx, tx, scope, item)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, item.run); err != nil {
		return false, err
	}
	var active, confirmed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.node_attempts WHERE run_id=$1 AND finished_at IS NULL),
 EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1 AND namespace=$2 AND reconciled_at IS NOT NULL)`, item.run, scope.Namespace).Scan(&active, &confirmed)
	if err != nil || active {
		return false, err
	}
	if !confirmed {
		return false, ErrEvolutionConsumerStopUnknown
	}
	return true, nil
}

func evolutionConsumerEvaluationTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, item evolutionConsumer) (bool, error) {
	var running bool
	err := tx.QueryRow(ctx, `SELECT r.status='running' OR EXISTS(SELECT 1 FROM pangu.node_attempts a WHERE a.finished_at IS NULL
 AND (a.run_id=r.run_id OR a.node_run_id IN (SELECT n.node_run_id FROM pangu.evolution_node_runs n WHERE n.request_id=r.request_id)))
 FROM pangu.evolution_case_runtime r WHERE r.request_id=$1 AND r.run_id=$2`, item.request, item.run).Scan(&running)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrEvolutionConsumerStopUnknown
	}
	if err != nil || running {
		return false, err
	}
	scope.ActorID = item.actor
	proof, err := (EvolutionEvidence{}).ReadStopTx(ctx, tx, scope, item.request)
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, evolution.ErrChecker) {
		return false, ErrEvolutionConsumerStopUnknown
	}
	if err != nil {
		return false, err
	}
	if !proof.StopConfirmed || proof.RunID != item.run || proof.RequestID != item.request {
		return false, ErrEvolutionConsumerStopUnknown
	}
	return true, nil
}
