// Package learning supplies database-backed trust boundaries for experience.
package learning

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

// NewStore never trusts caller-supplied authorization or verification results.
func NewStore() experience.Store {
	return experience.Store{Authorize: authorize, VerifySource: verifySource,
		VerifyClaim: verifyClaim, VerifyBenefit: verifyBenefit, VerifyIntent: verifyIntent}
}

func verifyClaim(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, activation experience.Activation) error {
	return computeClaimTx(ctx, tx, scope, version, activation)
}

func verifyBenefit(context.Context, pgx.Tx, experience.Scope, experience.Version, experience.DeliveryChange) error {
	return experience.ErrForbidden
}

func verifyIntent(ctx context.Context, tx pgx.Tx, scope experience.Scope, input experience.IntentInput) error {
	if !validID(input.SourceRunID) || input.Purpose != "business" || !input.Terminal || !input.StopConfirmed {
		return experience.ErrForbidden
	}
	run, err := businessRunTx(ctx, tx, scope, input.SourceRunID)
	if err != nil {
		return err
	}
	policy, err := policyTx(ctx, tx, scope)
	if err != nil {
		return err
	}
	if run.EvidenceRevision != input.EvidenceRevision || policy.Revision != input.PolicyRevision {
		return experience.ErrConflict
	}
	return nil
}
