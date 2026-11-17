package evolution

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrPublishedEvidence means retained offline proof is positively ineligible,
// not that a source read or SQL operation had an unknown outcome.
var ErrPublishedEvidence = errors.New("已发布版本的离线证据资格已失效")

// PublishedEvidenceTx is a forward-only, same-transaction serving fence. It does
// not require the evaluator to remain enabled and must never gate compensation.
func (s Service) PublishedEvidenceTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int) error {
	intent, policy, err := authorizedIntentTx(ctx, tx, scope, id, generation)
	if err != nil {
		return err
	}
	scope.ActorID = policy.ActorID
	return s.publishedForwardTx(ctx, tx, scope, intent, policy)
}

func (s Service) publishedForwardTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, policy experience.Policy) error {
	if err := publishedEvidenceTx(ctx, tx, scope, intent, policy); err != nil {
		return err
	}
	if _, err := s.recheckMemoriesTx(ctx, tx, scope, intent.Config); err != nil {
		return err
	}
	return rolloutPublicationUsable(ctx, tx, scope, intent)
}

func publishedEvidenceTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, policy experience.Policy) error {
	if intent.Publication == nil || intent.ExperimentID == "" || intent.ExperimentHash == "" {
		return errors.Join(ErrPublishedEvidence, ErrConflict)
	}
	frozen, err := evaluation.ReadExperimentTx(ctx, tx, scope.Namespace, scope.ProjectID, intent.ExperimentID)
	if err != nil {
		return publishedEvidenceError(err)
	}
	if frozen.Hash != intent.ExperimentHash || frozen.Generation != intent.Generation || frozen.SuiteID != intent.Config.Request.SuiteID {
		return errors.Join(ErrPublishedEvidence, ErrConflict)
	}
	suite, err := evaluation.ReadExperimentSuiteTx(ctx, tx, scope.Namespace, frozen.SuiteID)
	if err != nil {
		return publishedEvidenceError(err)
	}
	return publishedEvidenceError(verifySuite(policy, suite))
}

func publishedEvidenceError(err error) error {
	var sql *pgconn.PgError
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &sql) {
		return err
	}
	if errors.Is(err, evaluation.ErrConflict) || errors.Is(err, evaluation.ErrNotFound) || errors.Is(err, evaluation.ErrInvalid) || errors.Is(err, ErrChecker) || errors.Is(err, ErrForbidden) {
		return errors.Join(ErrPublishedEvidence, err)
	}
	return err
}

func publishedRejectTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, cause error) error {
	if !errors.Is(cause, ErrPublishedEvidence) {
		return cause
	}
	return rolloutBeginRollback(ctx, tx, scope, intent, "离线实验、suite来源或checker资格已失效", "insufficient")
}
