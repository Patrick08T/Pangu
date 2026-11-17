package evolution

import (
	"context"
	"slices"
	"strings"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/jackc/pgx/v5"
)

// CancelEvaluation reconciles pre-publication runs; an unknown stop retains every target claim.
func (s Service) CancelEvaluation(ctx context.Context, scope Scope, id string, generation int, reason string) (*Intent, error) {
	if strings.TrimSpace(reason) == "" || len(reason) > 1024 {
		return nil, ErrInvalid
	}
	var out *Intent
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, p, err := rolloutRecoveryTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		scope.ActorID = p.ActorID
		if intent.Publication != nil {
			return ErrConflict
		}
		if intent.State == "insufficient" {
			out = &intent.Intent
			return nil
		}
		if err = s.cancelEvaluationTx(ctx, tx, scope, intent, reason); err != nil {
			return err
		}
		out = &intent.Intent
		return nil
	})
	return out, err
}

func (s Service) cancelEvaluationTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, reason string) error {
	if !slices.Contains([]string{"preparing", "evaluating", "passed", "reconciling"}, intent.State) || intent.Publication != nil {
		return ErrConflict
	}
	if intent.State != "reconciling" {
		if _, err := tx.Exec(ctx, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{_cancel}',$2) WHERE id=$1`, intent.ID, jsonBytes(map[string]string{"reason": reason})); err != nil {
			return err
		}
		if err := setIntentStateTx(ctx, tx, intent, "reconciling"); err != nil {
			return err
		}
	}
	stopped, err := s.cancelExecutionsTx(ctx, tx, scope, intent)
	if err != nil {
		return err
	}
	if !stopped {
		return nil
	}
	if err = evaluation.InvalidateExperimentTx(ctx, tx, scope.Namespace, intent.ExperimentID, intent.Generation); err != nil {
		return err
	}
	if intent.Config.Request.Kind != "memory_benefit" {
		if err = catalog.ClearExperimentCandidateTx(ctx, tx, scope.Namespace, scope.ProjectID, intent.ExperimentID, func(ctx context.Context, tx pgx.Tx) error { return verifyClaimsTx(ctx, tx, scope, intent) }); err != nil {
			return err
		}
	}
	if err = setIntentStateTx(ctx, tx, intent, "insufficient"); err != nil {
		return err
	}
	return releaseIntentClaimsTx(ctx, tx, scope, intent)
}
