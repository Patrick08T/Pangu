package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/identity"
)

// StartEvolution is the human/API enqueue boundary, not a background identity fallback.
// It durably reserves work; periodic maintenance drives execution.
func (b Backend) StartEvolution(ctx context.Context, scope evolution.Scope, request evolution.CandidateRequest) (*evolution.Intent, error) {
	if b.client == nil {
		return nil, ErrNotReady
	}
	if scope.Namespace != b.Namespace() {
		return nil, evolution.ErrForbidden
	}
	actor := identity.Current(ctx)
	if actor.ID == "" || actor.ID != scope.ActorID || actor.Namespace != scope.Namespace || actor.TokenID != "" {
		return nil, evolution.ErrForbidden
	}
	return b.EvolutionService().ReserveCandidate(ctx, scope, request)
}
