package scheduler

import (
	"context"
	"errors"
	"sort"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/jackc/pgx/v5"
)

func (b Backend) experienceEraseRun(ctx context.Context, root experienceErasureBinding) (string, error) {
	var closure []experienceErasureBinding
	var refs []experience.Reference
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		closure, refs, err = experienceErasureCancelTx(ctx, tx, root)
		return err
	})
	if err != nil {
		return "Deferred", err
	}
	if len(refs) == 0 {
		return "", nil
	}
	for _, item := range closure {
		if err = b.experienceErasureStop(ctx, item); err != nil {
			auditErr := evolutionTransaction(ctx, func(tx pgx.Tx) error { return experienceErasureEventTx(ctx, tx, root, "WaitingStop", refs) })
			return "WaitingStop", errors.Join(err, auditErr)
		}
	}
	// A pending fork's only proof may live in its accepted snapshot. Keep the
	// proved origin intact until all continuations have been physically cleared.
	sort.SliceStable(closure, func(i, j int) bool { return closure[j] == root && closure[i] != root })
	for _, item := range closure {
		verify := func(check context.Context, tx pgx.Tx) error {
			return experienceErasureGuard(check, tx, root, item, refs)
		}
		if err = workspace.EraseExperienceRun(ctx, item.namespace, item.project, item.request, item.run, verify); err != nil {
			return "Deferred", err
		}
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := experienceErasureEventTx(ctx, tx, root, "PayloadErased", refs); err != nil {
			return err
		}
		return experienceErasureEventTx(ctx, tx, root, "Purged", refs)
	})
	return "Purged", err
}
