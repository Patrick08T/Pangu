package evolution

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// RetentionTarget 只携带不可变身份；守卫不得依赖已合法删除的私有正文。
type RetentionTarget struct {
	IntentID     string
	Generation   int
	ExperimentID string
	VersionIDs   []string
}

// RetentionGuard 同事务确认所有消费者停止或所有未来引用解除；不得外部I/O。
type RetentionGuard func(context.Context, pgx.Tx, Scope, RetentionTarget, string) (bool, error)

type RetentionResult struct {
	Due              bool
	Deferred         bool
	CandidateCleared bool
	VersionsCleared  int
	State            string
	Reason           string
}

func (s Service) ReconcileRetention(ctx context.Context, scope Scope, id string, generation int, guard RetentionGuard) (RetentionResult, error) {
	var out RetentionResult
	if scope.ActorID != "" {
		return out, ErrForbidden
	}
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, policy, err := retentionLoadTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		out.State = intent.State
		out.Reason, err = retentionReasonTx(ctx, tx, scope, intent, policy)
		if err != nil || out.Reason == "" {
			return err
		}
		out.Due = true
		out.Deferred = true
		scope.ActorID = policy.ActorID
		target := retentionTarget(intent)
		if err = retentionStopFenceTx(ctx, tx, intent, out.Reason, true); err != nil {
			return err
		}
		if err = retentionRevokeTx(ctx, tx, scope, intent, target, out.Reason); err != nil {
			return err
		}
		out.State = intent.State
		ready, err := retentionCheck(ctx, tx, scope, target, "stopped", guard)
		if err != nil || !ready {
			return err
		}
		if err = retentionStopFenceTx(ctx, tx, intent, out.Reason, false); err != nil {
			return err
		}
		if intent.Publication == nil {
			ready, err = retentionCheck(ctx, tx, scope, target, "unreferenced", guard)
			if err != nil || !ready {
				return err
			}
		}
		candidateBefore, err := retentionCandidatePresentTx(ctx, tx, target.ExperimentID)
		if err != nil {
			return err
		}
		ready, err = s.retentionSettleTx(ctx, tx, scope, intent, out.Reason)
		out.State = intent.State
		if err != nil || !ready {
			return err
		}
		ready, err = retentionUnreferencedTx(ctx, tx, scope, target, guard)
		if err != nil || !ready {
			return err
		}
		if err = retentionClearTx(ctx, tx, scope, intent, target, &out); err != nil {
			return err
		}
		out.CandidateCleared = candidateBefore
		out.Deferred = false
		return nil
	})
	if err != nil {
		return RetentionResult{}, err
	}
	return out, nil
}

func retentionTarget(intent *intentRecord) RetentionTarget {
	target := RetentionTarget{IntentID: intent.ID, Generation: intent.Generation, ExperimentID: intent.ExperimentID}
	if intent.Publication != nil {
		if intent.Publication.AgentID != "" {
			target.VersionIDs = append(target.VersionIDs, intent.Publication.AgentID)
		}
		if intent.Publication.FlowID != "" {
			target.VersionIDs = append(target.VersionIDs, intent.Publication.FlowID)
		}
	}
	return target
}

func retentionCheck(ctx context.Context, tx pgx.Tx, scope Scope, target RetentionTarget, phase string, guard RetentionGuard) (bool, error) {
	if guard == nil {
		return false, nil
	}
	return guard(ctx, tx, scope, target, phase)
}
