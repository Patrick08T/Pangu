package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

type evolutionConsumerCancel func(context.Context, evolution.Scope, evolutionConsumer) error

func (b Backend) ReconcileEvolutionRetention(ctx context.Context, scope evolution.Scope, id string, generation int) (evolution.RetentionResult, error) {
	if scope.Namespace != b.Namespace() {
		return evolution.RetentionResult{}, evolution.ErrForbidden
	}
	return reconcileEvolutionRetention(ctx, scope, id, generation, b.cancelEvolutionConsumer)
}

func reconcileEvolutionRetention(ctx context.Context, scope evolution.Scope, id string, generation int, cancel evolutionConsumerCancel) (evolution.RetentionResult, error) {
	service := newEvolutionService()
	var target evolution.RetentionTarget
	capture := func(_ context.Context, _ pgx.Tx, _ evolution.Scope, value evolution.RetentionTarget, phase string) (bool, error) {
		if phase != "stopped" {
			return false, evolution.ErrInvalid
		}
		target = value
		return false, nil
	}
	// Commit revocation/fencing first, so later RPC or unknown-proof failures cannot
	// roll the lifecycle back to runnable. No network request occurs in either tx.
	out, err := service.ReconcileRetention(ctx, scope, id, generation, capture)
	if err != nil || !out.Due {
		return out, err
	}
	if target.IntentID == "" {
		return out, ErrEvolutionConsumerStopUnknown
	}
	if err = evolutionCancelConsumers(ctx, scope, target, cancel); err != nil {
		return out, err
	}
	final, err := service.ReconcileRetention(ctx, scope, id, generation, EvolutionRetentionGuard)
	if err != nil {
		return out, err
	}
	return final, nil
}

func evolutionCancelConsumers(ctx context.Context, scope evolution.Scope, target evolution.RetentionTarget, cancel evolutionConsumerCancel) error {
	after := ""
	var failures []error
	for {
		items, err := evolutionConsumerPage(ctx, scope, target, after)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		if len(items) == 0 {
			return errors.Join(failures...)
		}
		for _, item := range items {
			if err = evolutionCancelConsumer(ctx, scope, item, cancel); err != nil {
				failures = append(failures, err)
			}
			after = item.request
		}
	}
}

func evolutionConsumerPage(ctx context.Context, scope evolution.Scope, target evolution.RetentionTarget, after string) ([]evolutionConsumer, error) {
	var out []evolutionConsumer
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := evolutionConsumerTargetTx(ctx, tx, scope, target); err != nil {
			return err
		}
		versions, err := evolutionConsumerVersionsTx(ctx, tx, scope, target.VersionIDs)
		if err != nil {
			return err
		}
		query := `SELECT * FROM (` + evolutionConsumersSQL + `) AS c(request,run,evaluation,actor) WHERE c.request>$6 ORDER BY c.request LIMIT 64`
		rows, err := tx.Query(ctx, query, scope.Namespace, scope.ProjectID, versions, target.IntentID, target.Generation, after)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item evolutionConsumer
			if err = rows.Scan(&item.request, &item.run, &item.evaluation, &item.actor); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func evolutionCancelConsumer(ctx context.Context, scope evolution.Scope, item evolutionConsumer, cancel evolutionConsumerCancel) error {
	if item.run == "" {
		return ErrEvolutionConsumerStopUnknown
	}
	if err := evolutionConsumerRequestCancel(ctx, scope, item); err != nil {
		return err
	}
	var closure []evolutionConsumer
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		closure, err = evolutionConsumerClosureTx(ctx, tx, scope, item)
		return err
	})
	if err != nil {
		return err
	}
	var failures []error
	for _, child := range closure {
		var stopped bool
		proofErr := evolutionTransaction(ctx, func(tx pgx.Tx) error {
			var err error
			stopped, err = evolutionConsumerStoppedTx(ctx, tx, scope, child)
			return err
		})
		if proofErr != nil && !errors.Is(proofErr, ErrEvolutionConsumerStopUnknown) {
			failures = append(failures, proofErr)
			continue
		}
		if stopped {
			continue
		}
		if child.run == "" || cancel == nil {
			failures = append(failures, ErrEvolutionConsumerStopUnknown)
			continue
		}
		if err = cancel(ctx, scope, child); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
