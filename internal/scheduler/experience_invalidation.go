package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
)

const experienceApprovalInvalidationReason = "审批依赖的经验或成果证明已失效，须停止旧任务并重新规划审批"

// validateExperienceApproval audits only a server-persisted approval identity.
// Known invalid dependencies are durably invalidated/cancelled; SQL/context and
// unknown failures propagate without creating an invalidation decision.
func validateExperienceApproval(ctx context.Context, namespace, approvalID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gate, err := approval.Get(ctx, namespace, approvalID)
	if err != nil {
		return err
	}
	if actor := identity.Current(ctx); actor.ID != "" && (actor.ID != gate.OwnerID || actor.Namespace != namespace) {
		return identity.ErrForbidden
	}
	if gate.InvalidatedAt != nil {
		return cancelInvalidExperienceGate(ctx, namespace, gate)
	}
	if err = auditExperienceGate(ctx, namespace, gate); err == nil {
		return approval.CheckGeneration(ctx, namespace, gate.RunID, gate.NodeID, gate.ApprovalGeneration)
	}
	if !invalidExperienceDependency(err) {
		return err
	}
	_, err = approval.Invalidate(ctx, namespace, gate.RunID, gate.NodeID, gate.ApprovalGeneration, experienceApprovalInvalidationReason)
	if errors.Is(err, approval.ErrExpired) {
		// Retention loss forbids rebuilding, but must not leave consumed work running.
		if cancelErr := execution.RequestCancel(ctx, namespace, gate.RunID); cancelErr != nil {
			return errors.Join(err, cancelErr)
		}
	}
	if err != nil {
		return err
	}
	return cancelInvalidExperienceGate(ctx, namespace, gate)
}

func invalidExperienceDependency(err error) bool {
	return errors.Is(err, experience.ErrInvalidated) || errors.Is(err, experience.ErrNotFound) ||
		errors.Is(err, experience.ErrForbidden) || errors.Is(err, experience.ErrInvalid) || errors.Is(err, identity.ErrForbidden)
}

func cancelInvalidExperienceGate(ctx context.Context, namespace string, gate *approval.Gate) error {
	if err := execution.RequestCancel(ctx, namespace, gate.RunID); err != nil {
		return err
	}
	return experience.ErrInvalidated
}

// ReconcileExperienceApprovals scans namespace gates using the frozen submitter's
// current authority. Call before the existing cancellation/Stop reconciler.
func (b Backend) ReconcileExperienceApprovals(ctx context.Context) error {
	return reconcileExperienceApprovals(ctx, b.Namespace())
}
