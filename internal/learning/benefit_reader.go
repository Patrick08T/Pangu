package learning

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

// BenefitPlanReader must reconstruct current authority and compiled input from a
// server-owned persisted plan. Returning the original frozen spec is not proof.
type BenefitPlanReader func(context.Context, pgx.Tx, experience.Scope, string) (evaluation.ExperimentSpec, error)

// NewStoreWithBenefit wires a trusted same-transaction plan reader; nil fails closed.
func NewStoreWithBenefit(reader BenefitPlanReader) experience.Store {
	store := NewStore()
	store.VerifyBenefit = func(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, change experience.DeliveryChange) error {
		if reader == nil {
			return experience.ErrForbidden
		}
		return verifyBenefitWithPlan(ctx, tx, scope, version, change, reader)
	}
	return store
}
