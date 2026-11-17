package scheduler

import (
	"context"
	"slices"
	"time"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

// ReadStopTx verifies liveness from durable runtime identity only. Authorized erasure
// of plan, output, role, memory, checker, or source bodies cannot pin cancellation.
func (EvolutionEvidence) ReadStopTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, request string) (evolution.StopProof, error) {
	proof := evolution.StopProof{RequestID: request}
	var actor string
	var started time.Time
	var finished *time.Time
	var active int
	err := tx.QueryRow(ctx, `SELECT r.run_id::text,r.status,r.started_at,r.finished_at,r.stop_confirmed,
 e.generation,e.input_hash,i.experiment_hash,p.actor_id::text,
 (SELECT count(*) FROM pangu.node_attempts a WHERE a.finished_at IS NULL AND (a.run_id=r.run_id OR
 a.node_run_id IN (SELECT n.node_run_id FROM pangu.evolution_node_runs n WHERE n.request_id=r.request_id)))
 FROM pangu.evolution_executions e JOIN pangu.evolution_intents i ON i.id=e.intent_id AND i.generation=e.generation
 JOIN pangu.experience_policies p ON p.namespace=i.namespace AND p.project_id=i.project_id AND p.revision=i.policy_revision
 JOIN pangu.evolution_case_runtime r ON r.request_id=e.request_id
 JOIN pangu.submissions s ON s.id=e.request_id AND s.run_id=r.run_id AND s.namespace=i.namespace AND s.project_id=i.project_id
 WHERE e.request_id=$1 AND i.namespace=$2 AND i.project_id=$3 AND s.snapshot->>'purpose'='evaluation'
 FOR SHARE OF r,s`, request, scope.Namespace, scope.ProjectID).Scan(&proof.RunID, &proof.Status, &started, &finished, &proof.StopConfirmed, &proof.Generation, &proof.ExecutionInputHash, &proof.ExperimentHash, &actor, &active)
	if err != nil {
		return proof, err
	}
	if actor == "" || actor != scope.ActorID {
		return proof, evolution.ErrForbidden
	}
	if !proof.StopConfirmed || finished == nil || active != 0 || !slices.Contains([]string{"completed", "failed", "timeout", "infrastructure_error", "cancelled"}, proof.Status) {
		return proof, evolution.ErrChecker
	}
	proof.SourceHash = evolutionHash(struct {
		Proof             evolution.StopProof
		Started, Finished time.Time
	}{proof, started, *finished})
	return proof, nil
}
