package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/jackc/pgx/v5"
)

func (b Backend) evolutionRecoverCase(ctx context.Context, s evolution.Scope, request, run string) (bool, error) {
	root, err := getHatchetRun(ctx, run)
	if err != nil {
		return false, err
	}
	expected, err := registeredWorkflowID(ctx, b.client.NewWorkflow(EvolutionTaskName).GetName())
	if err != nil {
		return false, err
	}
	if root.Run.WorkflowId != expected || root.Run.ParentTaskExternalId != nil {
		return false, evolution.ErrConflict
	}
	if !terminal(runStatus(root.Run.Status)) {
		return false, nil
	}
	for _, task := range root.Tasks {
		if !terminal(runStatus(task.Status)) {
			return false, nil
		}
	}
	// A terminal engine receipt does not prove local work has stopped.
	return evolutionAbandonCase(ctx, s, request, run)
}

func evolutionAbandonCase(ctx context.Context, s evolution.Scope, request, run string) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	var locked bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "evolution-case:"+request).Scan(&locked)
	if err != nil || !locked {
		return false, err
	}
	active, err := execution.Active(ctx, run)
	if err != nil || active != 0 {
		return false, err
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `INSERT INTO pangu.evolution_case_runtime(request_id,run_id,status,finished_at,stop_confirmed) SELECT id,run_id,'infrastructure_error',clock_timestamp(),true FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND id=$3 AND run_id=$4 AND operation='evolution' AND snapshot->>'purpose'='evaluation' AND purged_at IS NULL ON CONFLICT(request_id) DO UPDATE SET status='infrastructure_error',finished_at=clock_timestamp(),stop_confirmed=true WHERE evolution_case_runtime.status='running' AND evolution_case_runtime.run_id=excluded.run_id`, s.Namespace, s.ProjectID, request, run)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return evolution.ErrConflict
		}
		return nil
	})
	return err == nil, err
}
