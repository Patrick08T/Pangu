package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

// RunEvolutionCase executes only a server-frozen, authorized no-tool case, never a project workspace.
func RunEvolutionCase(ctx context.Context, namespace, requestID, runID string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, "evolution-case:"+requestID); err != nil {
		return err
	}
	var scope evolution.Scope
	var plan evolution.ExecutionPlan
	var completed bool
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		var e error
		scope, plan, e = evolutionReadPlanTx(ctx, tx, namespace, requestID)
		if e != nil {
			return e
		}
		completed, e = evolutionBeginCase(ctx, tx, scope, plan, runID)
		return e
	})
	if err != nil || completed {
		return err
	}
	service := newEvolutionService()
	fresh, err := service.PrepareCase(ctx, scope, plan.Key)
	if err == nil && evolutionHash(evolutionEnvelope(*fresh)) != evolutionHash(evolutionEnvelope(plan)) {
		err = evolution.ErrConflict
	}
	var outputs map[string]workflow.NodeOutput
	if err == nil {
		outputs, err = evolutionRunFlow(ctx, scope, plan, runID, service)
	}
	status := "completed"
	if err != nil {
		status = "failed"
	}
	finishErr := evolutionFinishCase(context.WithoutCancel(ctx), requestID, runID, status, outputs)
	return errors.Join(err, finishErr)
}

func evolutionBeginCase(ctx context.Context, tx pgx.Tx, s evolution.Scope, p evolution.ExecutionPlan, run string) (bool, error) {
	if err := evolutionFence(ctx, tx, s, p); err != nil {
		return false, err
	}
	// Hatchet may start before RunNoWait returns; only its actual root identity binds this reserved request.
	if _, err := tx.Exec(ctx, `UPDATE pangu.submissions SET run_id=$4 WHERE id=$1 AND namespace=$2 AND project_id=$3 AND operation='evolution' AND run_id IS NULL AND purged_at IS NULL`, p.RequestID, s.Namespace, s.ProjectID, run); err != nil {
		return false, err
	}
	var bound string
	err := tx.QueryRow(ctx, `SELECT run_id::text FROM pangu.submissions WHERE id=$1 AND namespace=$2 AND project_id=$3 AND snapshot->>'purpose'='evaluation' AND purged_at IS NULL`, p.RequestID, s.Namespace, s.ProjectID).Scan(&bound)
	if err != nil {
		return false, err
	}
	if bound != run {
		return false, evolution.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evolution_case_runtime(request_id,run_id,status) VALUES($1,$2,'running') ON CONFLICT DO NOTHING`, p.RequestID, run)
	if err != nil {
		return false, err
	}
	var actual, status string
	var stopped bool
	err = tx.QueryRow(ctx, `SELECT run_id::text,status,stop_confirmed FROM pangu.evolution_case_runtime WHERE request_id=$1 FOR UPDATE`, p.RequestID).Scan(&actual, &status, &stopped)
	if err != nil {
		return false, err
	}
	if actual != run {
		return false, evolution.ErrConflict
	}
	if status != "running" {
		if !stopped {
			return false, evolution.ErrConflict
		}
		if status != "completed" {
			return true, evolution.ErrChecker
		}
		return true, nil
	}
	return false, nil
}

func evolutionRunFlow(ctx context.Context, s evolution.Scope, p evolution.ExecutionPlan, run string, service evolution.Service) (map[string]workflow.NodeOutput, error) {
	return workflow.RunAudited(p.Definition, nil, nil, nil, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		results := make([]workflow.NodeOutput, 0, len(inputs))
		for _, input := range inputs {
			fresh, err := service.PrepareCase(ctx, s, p.Key)
			if err != nil {
				return nil, err
			}
			if evolutionHash(evolutionEnvelope(*fresh)) != evolutionHash(evolutionEnvelope(p)) {
				return nil, evolution.ErrConflict
			}
			out, err := evolutionExecuteNode(ctx, s, p, input, run)
			if err != nil {
				return nil, err
			}
			results = append(results, out)
		}
		return results, nil
	})
}

func evolutionFinishCase(ctx context.Context, request, run, status string, outputs map[string]workflow.NodeOutput) error {
	active, err := execution.Active(ctx, run)
	if err != nil {
		return err
	}
	if active != 0 {
		return evolution.ErrConflict
	}
	return evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE pangu.evolution_case_runtime SET status=$3,outputs=$4,finished_at=clock_timestamp(),stop_confirmed=true WHERE request_id=$1 AND run_id=$2 AND status='running'`, request, run, status, evolutionJSON(outputs))
		return err
	})
}
