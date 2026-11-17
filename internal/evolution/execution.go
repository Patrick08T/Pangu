package evolution

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func (s Service) PrepareCase(ctx context.Context, scope Scope, key CaseKey) (*ExecutionPlan, error) {
	if s.Evidence == nil {
		return nil, ErrChecker
	}
	var out *ExecutionPlan
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, p, err := authorizedIntentTx(ctx, tx, scope, key.IntentID, key.Generation)
		if err != nil {
			return err
		}
		scope.ActorID = p.ActorID
		if intent.State != "evaluating" {
			return ErrConflict
		}
		live, err := s.liveExperimentTx(ctx, tx, scope, intent, p)
		if err != nil {
			return err
		}
		out, _, err = casePlanTx(ctx, tx, scope, key, live)
		if err != nil {
			return err
		}
		return reserveExecutionTx(ctx, tx, out)
	})
	return out, err
}

func (s Service) RecordCaseResult(ctx context.Context, scope Scope, key CaseKey, requestID string) error {
	if s.Evidence == nil {
		return ErrChecker
	}
	return transaction(ctx, func(tx pgx.Tx) error {
		intent, p, err := authorizedIntentTx(ctx, tx, scope, key.IntentID, key.Generation)
		if err != nil {
			return err
		}
		scope.ActorID = p.ActorID
		if intent.State != "evaluating" {
			return ErrConflict
		}
		live, err := s.liveExperimentTx(ctx, tx, scope, intent, p)
		if err != nil {
			return err
		}
		plan, actual, err := casePlanTx(ctx, tx, scope, key, live)
		if err != nil {
			return err
		}
		execution, err := executionTx(ctx, tx, key)
		if err != nil {
			return err
		}
		if execution.RequestID != requestID || execution.InputHash != plan.InputHash || execution.MaxCalls != plan.MaxCalls || execution.MaxTokens != plan.MaxTokens {
			return ErrConflict
		}
		plan.RequestID = requestID
		result, err := s.Evidence.ReadExecutionTx(ctx, tx, scope, requestID)
		if err != nil {
			return err
		}
		if err = validateMachineResult(plan, result); err != nil {
			return err
		}
		receipt, err := machineReceipt(plan, actual, result)
		if err != nil {
			return err
		}
		return storeMachineResultTx(ctx, tx, scope, plan, result, receipt)
	})
}
