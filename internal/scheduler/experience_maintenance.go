package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

type experienceMaintenanceState struct {
	drive     string
	retention string
	project   string
}

func experienceMaintenanceStep(ctx context.Context, work func(context.Context) error) error {
	check, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return work(check)
}

// Invalidation must persist cancellation before the ordinary Stop reconciler.
// Each phase has its own budget: an error/timeout never skips later phases.
func (b Backend) maintainExperienceBeforeCancel(ctx context.Context, state *experienceMaintenanceState) error {
	service := b.EvolutionService()
	memoryErr := experienceMaintenanceStep(ctx, func(check context.Context) error {
		_, err := learning.ReconcileMemory(check, b.Namespace(), service.CurrentBenefitExperimentTx)
		return err
	})
	approvalErr := experienceMaintenanceStep(ctx, b.ReconcileExperienceApprovals)
	retentionErr := experienceMaintenanceStep(ctx, func(check context.Context) error {
		return b.retireExperienceMaintenance(check, &state.retention)
	})
	erasureErr := experienceMaintenanceStep(ctx, b.ReconcileExperienceErasure)
	return errors.Join(memoryErr, approvalErr, retentionErr, erasureErr)
}

func (b Backend) maintainExperienceDrive(ctx context.Context, state *experienceMaintenanceState) error {
	return experienceMaintenanceStep(ctx, func(check context.Context) error {
		return b.driveExperienceMaintenance(check, b.Namespace(), &state.drive)
	})
}

func (b Backend) Retention(ctx context.Context, preview bool) ([]workspace.RetentionItem, error) {
	return b.retention(ctx, preview, &experienceMaintenanceState{})
}

func (b Backend) retention(ctx context.Context, preview bool, state *experienceMaintenanceState) ([]workspace.RetentionItem, error) {
	if preview {
		return b.retainWorkspaces(ctx, true)
	}
	intentErr := experienceMaintenanceStep(ctx, func(check context.Context) error {
		return b.retireExperienceMaintenance(check, &state.retention)
	})
	caseErr := experienceMaintenanceStep(ctx, b.ReconcileExperienceRetention)
	memoryErr := experienceMaintenanceStep(ctx, func(check context.Context) error {
		return purgeExperienceMemory(check, b.Namespace(), &state.project, false)
	})
	erasureErr := experienceMaintenanceStep(ctx, b.ReconcileExperienceErasure)
	items, workspaceErr := b.retainWorkspaces(ctx, false)
	return items, errors.Join(intentErr, caseErr, memoryErr, erasureErr, workspaceErr)
}
