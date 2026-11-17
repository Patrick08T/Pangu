package evolution

import (
	"context"
	"errors"
	"slices"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func (s Service) PrepareSwitch(ctx context.Context, scope Scope, id string, generation int, entryKey, direction string) (*EntrySnapshot, error) {
	var out *EntrySnapshot
	var denied error
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, policy, err := rolloutSwitchAuthority(ctx, tx, scope, id, generation, direction)
		if err != nil {
			return err
		}
		scope.ActorID = policy.ActorID
		if direction == "rollback" {
			out, err = s.rolloutRollbackTx(ctx, tx, scope, intent, entryKey)
		} else {
			if denied = s.publishedForwardTx(ctx, tx, scope, intent, policy); denied != nil {
				return publishedRejectTx(ctx, tx, scope, intent, denied)
			}
			out, err = s.rolloutPrepareTx(ctx, tx, scope, intent, entryKey, direction)
		}
		return err
	})
	if err == nil && denied != nil {
		return nil, denied
	}
	return out, err
}

func rolloutSwitchAuthority(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int, direction string) (*intentRecord, experience.Policy, error) {
	var policy experience.Policy
	if direction == "adopt" {
		return authorizedIntentTx(ctx, tx, scope, id, generation)
	}
	if direction != "rollback" {
		return nil, policy, ErrInvalid
	}
	intent, policy, err := rolloutRecoveryTx(ctx, tx, scope, id, generation)
	if err != nil {
		return nil, policy, err
	}
	if intent.State != "rolling_back" && intent.State != "reconciling" {
		return nil, policy, ErrConflict
	}
	return intent, policy, nil
}

func (s Service) CompleteSwitch(ctx context.Context, scope Scope, id string, generation int, entryKey, direction string) (*Intent, error) {
	var out *Intent
	var denied error
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, policy, err := rolloutSwitchAuthority(ctx, tx, scope, id, generation, direction)
		if err != nil {
			return err
		}
		scope.ActorID = policy.ActorID
		if _, err = rolloutEntry(intent, entryKey); err != nil {
			return err
		}
		if direction == "adopt" && intent.State != "switching" {
			return ErrConflict
		}
		if direction == "adopt" {
			if denied = s.publishedForwardTx(ctx, tx, scope, intent, policy); denied != nil {
				return publishedRejectTx(ctx, tx, scope, intent, denied)
			}
		}
		if err = s.rolloutCompleteTx(ctx, tx, scope, intent, entryKey, direction); err != nil {
			return err
		}
		out = &intent.Intent
		return nil
	})
	if err == nil && denied != nil {
		return nil, denied
	}
	return out, err
}

func (s Service) Reconcile(ctx context.Context, scope Scope, id string, generation int) (*Intent, error) {
	var out *Intent
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, policy, err := rolloutRecoveryTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		recoveryScope := scope
		recoveryScope.ActorID = policy.ActorID
		if slices.Contains([]string{"stable", "rolled_back", "insufficient"}, intent.State) {
			out = &intent.Intent
			return nil
		}
		if intent.Publication == nil {
			return ErrConflict
		}
		if intent.State != "rolling_back" && intent.State != "reconciling" {
			_, _, err = authorizedIntentTx(ctx, tx, scope, id, generation)
			if err == nil {
				err = s.publishedForwardTx(ctx, tx, recoveryScope, intent, policy)
			}
			if err == nil && intent.State == "switching" {
				err = s.rolloutReconcileAdopt(ctx, tx, recoveryScope, intent)
			}
			if errors.Is(err, ErrPublishedEvidence) {
				if err = publishedRejectTx(ctx, tx, recoveryScope, intent, err); err != nil {
					return err
				}
			} else if err != nil {
				if !rolloutRecoverable(err) {
					return err
				}
				if err = rolloutBeginRollback(ctx, tx, recoveryScope, intent, "授权、依赖或入口发生变化", "rolled_back"); err != nil {
					return err
				}
			}
		}
		if intent.State == "rolling_back" || intent.State == "reconciling" {
			if err = s.rolloutReconcileRollback(ctx, tx, recoveryScope, intent); err != nil {
				return err
			}
		}
		out = &intent.Intent
		return nil
	})
	return out, err
}

func (s Service) rolloutReconcileAdopt(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	confirmed := true
	for _, entry := range intent.Config.Entries {
		if _, err := s.rolloutPrepareTx(ctx, tx, scope, intent, entry.Target.Key(), "adopt"); err != nil {
			return err
		}
		if err := s.rolloutCompleteTx(ctx, tx, scope, intent, entry.Target.Key(), "adopt"); err != nil {
			return err
		}
		row, err := rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, entry.Target.Key(), "adopt")
		if err != nil {
			return err
		}
		if row.Status == "conflict" {
			return ErrConflict
		}
		if row.Status != "confirmed" {
			confirmed = false
		}
	}
	if !confirmed {
		return nil
	}
	if err := setIntentStateTx(ctx, tx, intent, "observing"); err != nil {
		return err
	}
	return tx.QueryRow(ctx, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp() WHERE id=$1 RETURNING observing_at`, intent.ID).Scan(&intent.StartedAt)
}

func (s Service) RequestRollback(ctx context.Context, scope Scope, id string, generation int, reason string) (*Intent, error) {
	return s.rolloutRequestRollback(ctx, scope, id, generation, reason)
}
func (s Service) Observe(ctx context.Context, scope Scope, id string, generation int, runID string) (*Intent, error) {
	return s.rolloutObserve(ctx, scope, id, generation, runID)
}
func (s Service) Advance(ctx context.Context, scope Scope, id string, generation int) (*Intent, error) {
	return s.rolloutAdvance(ctx, scope, id, generation)
}
