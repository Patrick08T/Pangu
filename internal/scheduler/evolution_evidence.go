package scheduler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type EvolutionEvidence struct{}

func (EvolutionEvidence) ReadExecutionTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, request string) (evolution.MachineResult, error) {
	result := evolution.MachineResult{RequestID: request}
	actual, plan, err := evolutionReadPlanTx(ctx, tx, scope.Namespace, request)
	if err != nil {
		return result, err
	}
	if actual.ProjectID != scope.ProjectID || actual.ActorID != scope.ActorID {
		return result, evolution.ErrForbidden
	}
	var raw []byte
	var started time.Time
	var finished *time.Time
	var active int
	err = tx.QueryRow(ctx, `SELECT r.run_id::text,r.status,r.outputs,r.started_at,r.finished_at,r.stop_confirmed,
 (SELECT count(*) FROM pangu.node_attempts a WHERE a.run_id=r.run_id AND a.finished_at IS NULL)
 FROM pangu.evolution_case_runtime r JOIN pangu.submissions s ON s.id=r.request_id AND s.run_id=r.run_id
 WHERE r.request_id=$1 AND s.namespace=$2 AND s.project_id=$3 AND s.snapshot->>'purpose'='evaluation' AND s.purged_at IS NULL`, request, scope.Namespace, scope.ProjectID).Scan(&result.RunID, &result.Status, &raw, &started, &finished, &result.StopConfirmed, &active)
	if err != nil {
		return result, err
	}
	if !result.StopConfirmed || active != 0 || finished == nil {
		return result, evolution.ErrChecker
	}
	outputs := map[string]workflow.NodeOutput{}
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &outputs); err != nil {
			return result, err
		}
	}
	result.NodeID, result.Text, err = evolutionFinalOutput(plan.Definition, outputs, result.Status == "completed")
	if err != nil {
		return result, err
	}
	if err = evolutionUsageTx(ctx, tx, &result); err != nil {
		return result, err
	}
	result.CompiledHash, result.RoleHash, result.MemoryHash, err = evolutionActualIdentityTx(ctx, tx, scope, plan)
	if err != nil {
		return result, err
	}
	result.Generation = plan.Key.Generation
	result.ExperimentHash = plan.Experiment.Hash
	result.ExecutionInputHash = plan.InputHash
	result.InputTextHash = evolutionTextHash(plan.Definition.Input)
	result.EnvironmentHash = plan.Environment.Hash
	result.DurationSeconds = finished.Sub(started).Seconds()
	result.DurationMeasured = result.DurationSeconds >= 0
	result.SourceHash = evolutionHash(struct {
		Result        evolution.MachineResult
		Outputs       map[string]workflow.NodeOutput
		Start, Finish time.Time
	}{result, outputs, started, *finished})
	return result, nil
}

func evolutionFinalOutput(d workflow.Definition, outputs map[string]workflow.NodeOutput, complete bool) (string, string, error) {
	parents := map[string]bool{}
	for _, n := range d.Nodes {
		for _, id := range n.DependsOn {
			parents[id] = true
		}
	}
	id := ""
	for _, n := range d.Nodes {
		if !parents[n.ID] {
			if id != "" {
				return "", "", evolution.ErrChecker
			}
			id = n.ID
		}
	}
	out, ok := outputs[id]
	if id == "" || (complete && (!ok || out.Skipped)) {
		return "", "", evolution.ErrChecker
	}
	return id, out.Text, nil
}

func evolutionUsageTx(ctx context.Context, tx pgx.Tx, r *evolution.MachineResult) error {
	var cost int64
	var unknown int64
	var currency, other string
	err := tx.QueryRow(ctx, `SELECT count(*),coalesce(sum(input_tokens+output_tokens),0),coalesce(sum(estimate_micros),0),
 count(*) FILTER(WHERE input_tokens IS NULL OR output_tokens IS NULL OR estimate_micros IS NULL OR finished_at IS NULL OR kind<>'model'),
 coalesce(min(price->>'currency'),''),coalesce(max(price->>'currency'),'') FROM pangu.usage_calls WHERE run_id=$1`, r.RunID).Scan(&r.Calls, &r.Tokens, &cost, &unknown, &currency, &other)
	if err != nil {
		return err
	}
	r.Cost = float64(cost) / 1000000
	r.Currency = currency
	r.CostMeasured = unknown == 0 && r.Calls > 0 && currency != "" && currency == other
	r.TokensMeasured = unknown == 0 && r.Calls > 0
	return nil
}

func (EvolutionEvidence) ReadObservationTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, run string) (evolution.BusinessEvidence, error) {
	return evolutionReadObservationTx(ctx, tx, scope, run)
}
