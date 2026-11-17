package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

type evolutionDispatchView struct {
	scope evolution.Scope
	plan  evolution.ExecutionPlan
	run   string
}

// evolutionDispatch finishes each SQL transaction before the external engine call.
// Runtime and per-node revalidation still fence revocation after the last commit.
func evolutionDispatch(ctx context.Context, namespace, request string, send func(context.Context, string) (string, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	initial, err := evolutionDispatchRead(ctx, namespace, request)
	if err != nil || initial.run != "" {
		return initial.run, err
	}
	service := newEvolutionService()
	fresh, err := service.PrepareCase(ctx, initial.scope, initial.plan.Key)
	if err != nil {
		return "", err
	}
	if fresh.RequestID != request || evolutionHash(evolutionEnvelope(*fresh)) != evolutionHash(evolutionEnvelope(initial.plan)) {
		return "", evolution.ErrConflict
	}
	current, err := evolutionDispatchRead(ctx, namespace, request)
	if err != nil || current.run != "" {
		return current.run, err
	}
	if current.scope != initial.scope || evolutionHash(evolutionEnvelope(current.plan)) != evolutionHash(evolutionEnvelope(*fresh)) {
		return "", evolution.ErrConflict
	}
	return send(ctx, request)
}

func evolutionDispatchRead(ctx context.Context, namespace, request string) (evolutionDispatchView, error) {
	var out evolutionDispatchView
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		out.scope, out.plan, err = evolutionReadPlanTx(ctx, tx, namespace, request)
		if err != nil {
			return err
		}
		if out.plan.RequestID != request {
			return evolution.ErrConflict
		}
		if err = evolutionFence(ctx, tx, out.scope, out.plan); err != nil {
			return err
		}
		out.run, err = evolutionDispatchRequestTx(ctx, tx, out.scope, out.plan)
		return err
	})
	if err != nil {
		out.run = ""
	}
	return out, err
}
