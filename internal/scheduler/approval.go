package scheduler

import (
	"context"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

const approvalEvent = "pangu-approval-v1"

func waitApproval(ctx hatchet.DurableContext, client *hatchet.Client, request string, d workflow.Definition, input *workflow.NodeInput, outputs map[string]workflow.NodeOutput, execute func([]workflow.NodeInput) ([]workflow.NodeOutput, error)) error {
	if input.Node.Approval == nil {
		return nil
	}
	runtime, err := loadNodeExperience(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), request, ctx.WorkflowRunId(), d.ProjectID)
	if err != nil {
		return err
	}
	for round := 1; ; round++ {
		fresh, err := rebindApprovalInput(d, *input, outputs, round)
		if err != nil {
			return err
		}
		*input = fresh
		if err := runtime.Prepare(ctx.GetContext(), input); err != nil {
			return err
		}
		if err := allowDispatch(ctx.GetContext(), ctx.WorkflowRunId()); err != nil {
			return err
		}
		gate, err := approval.Request(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), request, ctx.WorkflowRunId(), *input)
		if err != nil {
			return hatchet.NewNonRetryableError(err)
		}
		event := hatchet.UserEventCondition(client.NewWorkflow(approvalEvent).GetName(), "true", hatchet.WithEventScope(gate.ID), hatchet.WithConsiderEventsSince(gate.CreatedAt))
		// 持久等待人工决定；Worker 重启后恢复同一审批事件。
		if gate.InheritedFrom == "" {
			if _, err = ctx.WaitFor(event); err != nil {
				return err
			}
		}
		if err := validateExperienceApproval(ctx.GetContext(), runtime.namespace, gate.ID); err != nil {
			return err
		}
		current, err := approval.Get(ctx.GetContext(), runtime.namespace, gate.ID)
		if err != nil {
			return err
		}
		if err = allowDispatch(ctx.GetContext(), ctx.WorkflowRunId()); err != nil {
			return err
		}
		if current.Status == "changes_requested" {
			if err = revisePlan(d, input.Node, *current, outputs, execute); err != nil {
				return err
			}
			continue
		}
		if current.Status != "approved" && current.Status != "consumed" {
			return hatchet.NewNonRetryableError(fmt.Errorf("节点 %s 审批未通过，状态：%s", input.Node.ID, current.Status))
		}
		input.ApprovalID = gate.ID
		if current.Execution != nil && len(current.Execution.Changes) > 0 && current.InheritedFrom == "" {
			return continuePlan(ctx, client, d, *current, outputs)
		}
		return nil
	}
}

func revisePlan(d workflow.Definition, node workflow.Node, gate approval.Gate, outputs map[string]workflow.NodeOutput, execute func([]workflow.NodeInput) ([]workflow.NodeOutput, error)) error {
	layers, err := d.Layers()
	if err != nil {
		return err
	}
	original, err := revisionOrigin(gate)
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, id := range node.Approval.RevisionNodes {
		selected[id] = true
	}
	feedback := &workflow.ReviewFeedback{ApprovalID: gate.ID, NodeID: node.ID, Round: gate.Round + 1, Reason: gate.Reason, Items: gate.Items, PreviousReview: *gate.Review}
	for _, layer := range layers {
		for _, n := range layer {
			if !selected[n.ID] {
				continue
			}
			input, err := d.BindInput(d.NodeInput(n, outputs))
			if err != nil {
				return err
			}
			if selected, err := d.Selected(input); err != nil || !selected {
				return fmt.Errorf("修订节点 %s 不满足条件: %v", n.ID, err)
			}
			input.RequestID = original.RequestID
			input.ApprovalGeneration = original.ApprovalGeneration
			input.Iteration = original.Iteration
			input.ReviewFeedback = feedback
			results, err := execute([]workflow.NodeInput{input})
			if err != nil {
				return fmt.Errorf("纠偏修订 %s 失败: %w", n.ID, err)
			}
			if len(results) != 1 {
				return fmt.Errorf("纠偏修订 %s 缺少结果", n.ID)
			}
			if err = workflow.ValidateOutput(n, &results[0]); err != nil {
				return err
			}
			outputs[n.ID] = results[0]
		}
	}
	return nil
}

func (b Backend) ReconcileApprovals(ctx context.Context) error {
	ids, err := approval.PendingEvents(ctx, b.Namespace())
	if err != nil {
		return err
	}
	for _, id := range ids {
		gate, err := approval.Get(ctx, b.Namespace(), id)
		if err != nil {
			return err
		}
		if err = b.client.Events().Push(ctx, approvalEvent, map[string]string{"approval_id": id}, hatchet.WithFilterScope(&id)); err != nil {
			return err
		}
		if err = approval.SignalledState(ctx, b.Namespace(), gate); err != nil {
			return err
		}
	}
	return nil
}
