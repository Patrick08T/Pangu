package evolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func benefitGuardError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{ErrForbidden, ErrConflict, ErrInvalid, ErrNotFound, ErrIsolation, ErrChecker, ErrBudget, experience.ErrForbidden, experience.ErrConflict, experience.ErrInvalid, experience.ErrNotFound, experience.ErrInvalidated, catalog.ErrInvalid, catalog.ErrConflict, catalog.ErrArchived, catalog.ErrNotFound, evaluation.ErrInvalid, evaluation.ErrConflict, evaluation.ErrNotFound, pgx.ErrNoRows} {
		if errors.Is(err, known) {
			return fmt.Errorf("benefit plan guard (%v): %w", err, experience.ErrInvalidated)
		}
	}
	return err
}

// CurrentBenefitExperimentTx reconstructs current identity, including after evaluation has terminated.
func (s Service) CurrentBenefitExperimentTx(ctx context.Context, tx pgx.Tx, input experience.Scope, experimentID string) (evaluation.ExperimentSpec, error) {
	current, err := s.currentBenefitExperimentTx(ctx, tx, input, experimentID)
	return current, benefitGuardError(err)
}

func (s Service) currentBenefitExperimentTx(ctx context.Context, tx pgx.Tx, input experience.Scope, experimentID string) (evaluation.ExperimentSpec, error) {
	scope := Scope{Namespace: input.Namespace, ProjectID: input.ProjectID, ActorID: input.ActorID}
	var empty evaluation.ExperimentSpec
	if err := lockScope(ctx, tx, scope); err != nil {
		return empty, err
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2 AND experiment_id=$3`, scope.Namespace, scope.ProjectID, experimentID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, ErrNotFound
	}
	if err != nil {
		return empty, err
	}
	intent, err := readIntentTx(ctx, tx, scope, id)
	if err != nil {
		return empty, err
	}
	p, err := policyTx(ctx, tx, scope, intent.PolicyRevision)
	if err != nil {
		return empty, err
	}
	scope.ActorID = p.ActorID
	if err = p.Validate(); err != nil {
		return empty, err
	}
	if err = validatePolicy(p, intent.Config.Request); err != nil {
		return empty, err
	}
	if intent.Config.Request.Kind != "memory_benefit" || (intent.State != "evaluating" && intent.State != "passed" && intent.State != "stable") {
		return empty, ErrConflict
	}
	if intent.State != "stable" {
		if err = verifyClaimsTx(ctx, tx, scope, intent); err != nil {
			return empty, err
		}
	}
	live, err := s.liveExperimentTx(ctx, tx, scope, intent, p)
	if err != nil {
		return empty, err
	}
	return live.Frozen.ExperimentSpec, nil
}

func finishBenefitTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	var pending bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_executions e LEFT JOIN pangu.evolution_machine_results r USING(request_id) WHERE e.intent_id=$1 AND e.generation=$2 AND (e.status='reserved' OR r.request_id IS NULL OR NOT coalesce((r.result->>'StopConfirmed')::boolean,false)))`, intent.ID, intent.Generation).Scan(&pending)
	if err != nil {
		return err
	}
	if pending {
		return ErrConflict
	}
	if err = setIntentStateTx(ctx, tx, intent, "stable"); err != nil {
		return err
	}
	return releaseIntentClaimsTx(ctx, tx, scope, intent)
}

func releaseIntentClaimsTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	tag, err := tx.Exec(ctx, `DELETE FROM pangu.evolution_target_claims WHERE namespace=$1 AND project_id=$2 AND intent_id=$3 AND generation=$4`, scope.Namespace, scope.ProjectID, intent.ID, intent.Generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != int64(len(intent.Targets)) {
		return ErrConflict
	}
	return nil
}
