package scheduler

import (
	"fmt"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func resolveEvidence(ctx hatchet.DurableContext, client *hatchet.Client, jobs []*nodeDispatch, outputs []workflow.NodeOutput) error {
	for _, job := range jobs {
		for round := 1; len(outputs[job.index].Blockers) > 0; round++ {
			if !job.input.Node.ReadOnly || job.input.Node.Approval != nil {
				return fmt.Errorf("补证复验只允许无实施审批的只读节点")
			}
			gate, err := waitEvidence(ctx, client, job.input, outputs[job.index])
			if err != nil {
				return err
			}
			job.input.EvidenceRetry = &workflow.EvidenceRetry{ApprovalID: gate.ID, Round: round, Reason: gate.Reason, Items: gate.Items}
			job.retry = 0
			if err = dispatchNode(ctx, job); err != nil {
				return err
			}
			if err = collectNode(ctx, job, outputs); err != nil {
				return err
			}
			if err = retryNodes(ctx, []*nodeDispatch{job}, outputs); err != nil {
				return err
			}
		}
	}
	return nil
}

func waitEvidence(ctx hatchet.DurableContext, client *hatchet.Client, input workflow.NodeInput, out workflow.NodeOutput) (*approval.Gate, error) {
	namespace := client.NewWorkflow(WorkflowName).GetName()
	request, err := submission.ForRun(ctx.GetContext(), namespace, ctx.WorkflowRunId())
	if err != nil {
		return nil, err
	}
	runtime, err := loadNodeExperience(ctx.GetContext(), namespace, request.ID, ctx.WorkflowRunId(), input.ProjectID)
	if err != nil {
		return nil, err
	}
	gateInput, err := runtime.EvidenceApprovalInput(ctx.GetContext(), input, out)
	if err != nil {
		return nil, err
	}
	gate, err := approval.Request(ctx.GetContext(), namespace, request.ID, ctx.WorkflowRunId(), gateInput)
	if err != nil {
		return nil, err
	}
	event := hatchet.UserEventCondition(client.NewWorkflow(approvalEvent).GetName(), "true", hatchet.WithEventScope(gate.ID), hatchet.WithConsiderEventsSince(gate.CreatedAt))
	if _, err = ctx.WaitFor(event); err != nil {
		return nil, err
	}
	if err := validateExperienceApproval(ctx.GetContext(), namespace, gate.ID); err != nil {
		return nil, err
	}
	current, err := approval.Get(ctx.GetContext(), namespace, gate.ID)
	if err != nil {
		return nil, err
	}
	if err = allowDispatch(ctx.GetContext(), ctx.WorkflowRunId()); err != nil {
		return nil, err
	}
	if current.Repair != nil && (current.Repair.Status == "approved" || current.Repair.Status == "consumed") {
		item, err := approval.ContinueRepair(ctx.GetContext(), namespace, gate.ID)
		if err != nil {
			return nil, err
		}
		NewBackend(client).dispatch(ctx.GetContext(), item)
		return nil, workflow.ErrContinued
	}
	if current.Status != "approved" && current.Status != "consumed" && current.Status != "changes_requested" {
		return nil, fmt.Errorf("补证复验已结束：%s", current.Status)
	}
	if err = approval.Consume(ctx.GetContext(), ctx.WorkflowRunId(), gateInput, gate.ID); err != nil {
		return nil, err
	}
	return current, nil
}
