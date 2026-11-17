package evolution

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type executionRecord struct {
	RequestID, InputHash, Status string
	MaxCalls, MaxTokens          int64
}

func casePlanTx(ctx context.Context, tx pgx.Tx, scope Scope, key CaseKey, live *liveExperiment) (*ExecutionPlan, *evaluation.Case, error) {
	if (key.Arm != "baseline" && key.Arm != "candidate") || key.Repeat < 1 || key.Repeat > live.Suite.Rules.Repeats {
		return nil, nil, ErrInvalid
	}
	var selected evaluation.SuiteCase
	count := int64(0)
	for _, c := range live.Suite.Cases {
		if c.Partition != "development" {
			count++
			if c.ID == key.CaseID {
				selected = c
			}
		}
	}
	if selected.ID == "" {
		return nil, nil, ErrNotFound
	}
	actual, err := evaluation.LoadExperimentCaseTx(ctx, tx, scope.Namespace, live.Frozen.ID, key.CaseID)
	if err != nil {
		return nil, nil, err
	}
	if _, err = checkJSONResult(actual.Expected, actual.Criteria, actual.Expected); err != nil {
		return nil, nil, err
	}
	definition := live.Baseline
	if key.Arm == "candidate" {
		definition = live.Candidate
	}
	definition.Input = actual.Input
	if err = definition.ValidateInput(); err != nil {
		return nil, nil, err
	}
	total := count * 2 * int64(live.Suite.Rules.Repeats)
	inputHash, err := evaluation.ExecutionInputHash(live.Suite, live.Frozen, key.Arm, selected.ID, key.Repeat)
	if err != nil {
		return nil, nil, err
	}
	plan := &ExecutionPlan{Key: key, Experiment: *live.Frozen, Environment: live.Environment, Definition: definition, InputHash: inputHash, Memories: live.Memories, MaxCalls: int64(live.Frozen.Budget.MaxCalls) / total, MaxTokens: live.Frozen.Budget.MaxTokens / total}
	if live.Frozen.Kind == "memory_benefit" && key.Arm == "baseline" {
		plan.Memories = nil
	}
	if plan.MaxCalls < int64(len(definition.Nodes)) || plan.MaxTokens < 1 {
		return nil, nil, ErrBudget
	}
	return plan, actual, nil
}

func executionTx(ctx context.Context, tx pgx.Tx, key CaseKey) (executionRecord, error) {
	var out executionRecord
	err := tx.QueryRow(ctx, `SELECT request_id::text,input_hash,status,max_calls,max_tokens FROM pangu.evolution_executions WHERE intent_id=$1 AND generation=$2 AND arm=$3 AND case_id=$4 AND repeat=$5 FOR UPDATE`, key.IntentID, key.Generation, key.Arm, key.CaseID, key.Repeat).Scan(&out.RequestID, &out.InputHash, &out.Status, &out.MaxCalls, &out.MaxTokens)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func reserveExecutionTx(ctx context.Context, tx pgx.Tx, plan *ExecutionPlan) error {
	prior, err := executionTx(ctx, tx, plan.Key)
	if err == nil {
		if prior.Status != "reserved" || prior.InputHash != plan.InputHash || prior.MaxCalls != plan.MaxCalls || prior.MaxTokens != plan.MaxTokens {
			return ErrConflict
		}
		plan.RequestID = prior.RequestID
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	plan.RequestID = uuid.NewString()
	key := plan.Key
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evolution_executions(intent_id,generation,arm,case_id,repeat,input_hash,request_id,status,max_calls,max_tokens) VALUES($1,$2,$3,$4,$5,$6,$7,'reserved',$8,$9)`, key.IntentID, key.Generation, key.Arm, key.CaseID, key.Repeat, plan.InputHash, plan.RequestID, plan.MaxCalls, plan.MaxTokens)
	return err
}
