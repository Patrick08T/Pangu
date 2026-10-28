package scheduler

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
	"time"
)

const approvalEvent = "pangu-approval-v1"

func waitApprovals(ctx hatchet.DurableContext, client *hatchet.Client, request string, inputs []workflow.NodeInput) error {
	for i := range inputs {
		input := &inputs[i]
		if input.Node.Approval == nil {
			continue
		}
		if err := allowDispatch(ctx.GetContext(), ctx.WorkflowRunId()); err != nil {
			return err
		}
		gate, err := approval.Request(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), request, ctx.WorkflowRunId(), *input)
		if err != nil {
			return hatchet.NewNonRetryableError(err)
		}
		duration := gate.ExpiresAt.Sub(gate.CreatedAt)
		if duration < time.Millisecond {
			duration = time.Millisecond
		}
		event := hatchet.UserEventCondition(client.NewWorkflow(approvalEvent).GetName(), "true", hatchet.WithEventScope(gate.ID), hatchet.WithConsiderEventsSince(gate.CreatedAt))
		// 每次重放保持同一等待顺序，已注册计时器与事件由 Hatchet 恢复。
		if _, err = ctx.WaitFor(hatchet.OrCondition(event, hatchet.SleepCondition(duration))); err != nil {
			return err
		}
		current, err := approval.Get(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), gate.ID)
		if err != nil {
			return err
		}
		if current.Status != "approved" && current.Status != "consumed" {
			return hatchet.NewNonRetryableError(approval.ErrConflict)
		}
		input.ApprovalID = gate.ID
	}
	return nil
}

func (b Backend) ReconcileApprovals(ctx context.Context) error {
	ids, err := approval.PendingEvents(ctx, b.Namespace())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = b.client.Events().Push(ctx, approvalEvent, map[string]string{"approval_id": id}, hatchet.WithFilterScope(&id)); err != nil {
			return err
		}
		if err = approval.Signalled(ctx, b.Namespace(), id); err != nil {
			return err
		}
	}
	return nil
}
