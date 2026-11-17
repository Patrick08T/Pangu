package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/usage"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

// Observe only the verified, final output, before checkpoint/report publication.
func executeNodeObserved(ctx context.Context, item execution.Attempt, input workflow.NodeInput, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error), observe func(workflow.NodeInput, *workflow.NodeOutput) error) (workflow.NodeOutput, error) {
	ctx = usage.WithRun(ctx, item.RunID, input.Node.ID, item.NodeRunID, item.Attempt)
	input.ProcessScope = item.ProcessScope
	input.RunID, input.NodeRunID = item.RunID, item.NodeRunID
	call := func() (workflow.NodeOutput, error) {
		if input.Node.Approval != nil {
			if err := approval.Consume(ctx, item.RunID, input, input.ApprovalID); err != nil {
				return workflow.NodeOutput{}, err
			}
		}
		actual := input
		invoke := func(ctx context.Context, resolved workflow.NodeInput) (workflow.NodeOutput, error) {
			actual = resolved
			return execute(ctx, resolved)
		}
		out, err := callWithEvidence(ctx, item, input, invoke)
		if err == nil && input.Workspace != "" && input.Node.Executor == "codex" {
			out, err = verifyWorkspaceOutput(ctx, item, input, out)
		}
		out.Iteration = input.Iteration
		if err == nil && observe != nil {
			err = observe(actual, &out)
		}
		return out, err
	}
	if input.Node.Role != "" {
		return research.Persist(ctx, item.RunID, item.NodeRunID, input.ProjectID, call)
	}
	if input.Node.Executor != "codex" {
		return call()
	}
	return workspace.Execute(ctx, item.RunID, item.NodeRunID, item.Attempt, input.Node.ReadOnly, func(path string) (workflow.NodeOutput, error) {
		input.Workspace = path
		var err error
		input.Revision, err = workspace.RevisionFor(ctx, input)
		if err != nil {
			return workflow.NodeOutput{}, &workflow.Failure{Kind: "permission", Cause: err}
		}
		return call()
	})
}
