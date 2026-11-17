package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/usage"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func evolutionNodeKey(input workflow.NodeInput) string {
	key := input.Node.ID
	if input.Iteration != nil {
		key += fmt.Sprintf(":%s:%d", input.Iteration.ID, input.Iteration.Round)
	}
	return key
}

func evolutionExecuteNode(ctx context.Context, s evolution.Scope, p evolution.ExecutionPlan, input workflow.NodeInput, run string) (workflow.NodeOutput, error) {
	nodeRun := uuid.NewString()
	out := workflow.NodeOutput{}
	reused := false
	input, err := evolutionNodeInput(p, input, run, nodeRun)
	if err != nil {
		return out, err
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := evolutionFence(ctx, tx, s, p); err != nil {
			return err
		}
		var e error
		out, reused, e = evolutionBeginNode(ctx, tx, p, input)
		return e
	})
	if err != nil || reused {
		return out, err
	}
	attempt := execution.Attempt{RunID: run, NodeRunID: nodeRun, NodeID: input.Node.ID}
	if err = execution.Begin(ctx, &attempt); err != nil {
		return out, err
	}
	out, err = agent.Execute(usage.WithRun(ctx, run, input.Node.ID, nodeRun, attempt.Attempt), input)
	out.RunID = nodeRun
	attempt.Status = "succeeded"
	status := "completed"
	if err != nil {
		attempt.Status = "failed"
		attempt.Message = err.Error()
		status = "failed"
	}
	finishErr := execution.Finish(context.WithoutCancel(ctx), attempt)
	if finishErr != nil {
		return out, errors.Join(err, finishErr)
	}
	saveErr := evolutionTransaction(context.WithoutCancel(ctx), func(tx pgx.Tx) error {
		_, e := tx.Exec(context.WithoutCancel(ctx), `UPDATE pangu.evolution_node_runs SET status=$3,output=$4,finished_at=clock_timestamp() WHERE request_id=$1 AND node_key=$2 AND status='running' AND node_run_id=$5`, p.RequestID, evolutionNodeKey(input), status, evolutionJSON(out), nodeRun)
		return e
	})
	return out, errors.Join(err, saveErr)
}

func evolutionBeginNode(ctx context.Context, tx pgx.Tx, p evolution.ExecutionPlan, input workflow.NodeInput) (workflow.NodeOutput, bool, error) {
	var out workflow.NodeOutput
	hash, err := workflow.EffectiveInputHash(input)
	if err != nil {
		return out, false, err
	}
	var oldHash, status string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT input_hash,status,output FROM pangu.evolution_node_runs WHERE request_id=$1 AND node_key=$2 FOR UPDATE`, p.RequestID, evolutionNodeKey(input)).Scan(&oldHash, &status, &raw)
	if err == nil {
		if oldHash != hash || status != "completed" {
			return out, false, evolution.ErrConflict
		}
		if err = json.Unmarshal(raw, &out); err != nil {
			return out, false, err
		}
		return out, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, false, err
	}
	if err = evolutionBudgetTx(ctx, tx, input.RunID, p.MaxCalls, p.MaxTokens); err != nil {
		return out, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evolution_node_runs(request_id,node_key,input_hash,node_run_id,status) VALUES($1,$2,$3,$4,'running')`, p.RequestID, evolutionNodeKey(input), hash, input.NodeRunID)
	return out, false, err
}

func evolutionBudgetTx(ctx context.Context, tx pgx.Tx, run string, maxCalls, maxTokens int64) error {
	var calls, tokens, unknown int64
	err := tx.QueryRow(ctx, `SELECT count(*),coalesce(sum(input_tokens+output_tokens),0),count(*) FILTER(WHERE input_tokens IS NULL OR output_tokens IS NULL OR estimate_micros IS NULL OR finished_at IS NULL) FROM pangu.usage_calls WHERE run_id=$1`, run).Scan(&calls, &tokens, &unknown)
	if err != nil {
		return err
	}
	if unknown != 0 {
		return evolution.ErrChecker
	}
	if calls >= maxCalls || tokens >= maxTokens {
		return evolution.ErrBudget
	}
	return nil
}
