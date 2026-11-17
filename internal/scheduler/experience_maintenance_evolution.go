package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/evolution"
)

func (b Backend) retireExperienceMaintenance(ctx context.Context, cursor *string) error {
	if cursor == nil {
		return evolution.ErrInvalid
	}
	check, cancel := context.WithTimeout(ctx, experienceMaintenanceItemBudget)
	items, err := evolution.ListRetentionTargets(check, b.Namespace(), *cursor, experienceMaintenanceBatch)
	cancel()
	if err != nil {
		return err
	}
	var failures error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return errors.Join(failures, err)
		}
		check, cancel := context.WithTimeout(ctx, experienceMaintenanceItemBudget)
		_, err := b.ReconcileEvolutionRetention(check, item.Scope, item.ID, item.Generation)
		cancel()
		*cursor = item.ID
		if err != nil {
			failures = errors.Join(failures, fmt.Errorf("evolution retention %s: %w", item.ID, err))
		}
	}
	if len(items) < experienceMaintenanceBatch {
		*cursor = ""
	}
	return failures
}

// Reuse the driver's existing scoped discovery. Its policy revision join reads
// the accepted actor, never the current editor or an HTTP caller's identity.
func (b Backend) driveExperienceMaintenance(ctx context.Context, namespace string, cursor *string) error {
	if namespace == "" || cursor == nil {
		return evolution.ErrInvalid
	}
	check, cancel := context.WithTimeout(ctx, experienceMaintenanceItemBudget)
	items, err := evolutionPendingPage(check, namespace, *cursor)
	cancel()
	if err != nil {
		return err
	}
	if len(items) > experienceMaintenanceBatch {
		items = items[:experienceMaintenanceBatch]
	}
	var failures error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return errors.Join(failures, err)
		}
		check, cancel := context.WithTimeout(ctx, experienceMaintenanceItemBudget)
		err := b.DriveEvolution(check, item.Scope, item.ID, item.Generation)
		cancel()
		*cursor = item.ID
		if err != nil {
			failures = errors.Join(failures, fmt.Errorf("evolution drive %s: %w", item.ID, err))
		}
	}
	if len(items) < experienceMaintenanceBatch {
		*cursor = ""
	}
	return failures
}
