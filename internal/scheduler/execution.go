package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func runAgent(ctx hatchet.Context, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error), input workflow.NodeInput) (workflow.NodeOutput, error) {
	if err := input.Node.Validate(); err != nil {
		return workflow.NodeOutput{}, hatchet.NewNonRetryableError(err)
	}
	policy := input.Node.EffectivePolicy()
	nodeCtx, cancelNode := context.WithTimeout(ctx.GetContext(), time.Duration(policy.TimeoutSeconds)*time.Second)
	defer cancelNode()
	parent := ctx.ParentWorkflowRunId()
	if parent == nil {
		return workflow.NodeOutput{}, hatchet.NewNonRetryableError(errors.New("Agent 节点必须由 Pangu 根运行派生"))
	}
	item := execution.Attempt{RunID: *parent, NodeRunID: ctx.WorkflowRunId(), NodeID: input.Node.ID, Attempt: ctx.RetryCount() + 1}
	if input.Node.Executor == "codex" {
		item.ProcessScope = uuid.NewString()
	}
	if err := execution.Begin(nodeCtx, &item); err != nil {
		return workflow.NodeOutput{}, hatchet.NewNonRetryableError(err)
	}
	out, err := executeNode(nodeCtx, item, input, execute)
	if errors.Is(err, workflow.ErrStopUnconfirmed) {
		return out, hatchet.NewNonRetryableError(err)
	}
	item.Evidence, item.EvidenceTruncated = out.Evidence, out.EvidenceTruncated
	out.RunID, item.Status = ctx.WorkflowRunId(), "succeeded"
	var failure *workflow.Failure
	if nodeCtx.Err() != nil && !errors.As(err, &failure) {
		err = nodeCtx.Err()
	}
	retry := workflow.CanRetry(err) && ctx.RetryCount() < policy.Retries
	item.Category = workflow.FailureKind(err)
	if err != nil {
		item.Status = "failed"
		item.Message, item.Truncated = diagnostic.Text(err.Error(), 1024*1024)
		err = errors.New(item.Message)
	}
	if errors.Is(nodeCtx.Err(), context.Canceled) {
		item.Status, err = "cancelled", nodeCtx.Err()
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if recordErr := execution.Finish(stopCtx, item); recordErr != nil {
		return workflow.NodeOutput{}, hatchet.NewNonRetryableError(recordErr)
	}
	if err != nil && !retry {
		return out, hatchet.NewNonRetryableError(err)
	}
	return out, err
}

func executeNode(ctx context.Context, item execution.Attempt, input workflow.NodeInput, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	input.ProcessScope = item.ProcessScope
	input.RunID, input.NodeRunID = item.RunID, item.NodeRunID
	call := func() (workflow.NodeOutput, error) {
		out, err := execute(ctx, input)
		out.RunID = item.NodeRunID
		if err == nil {
			err = workflow.ValidateOutput(input.Node, &out)
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
		out, err := call()
		if err != nil {
			return out, err
		}
		command, err := workspace.Verification(ctx, item.RunID)
		if err != nil || len(command) == 0 {
			return out, err
		}
		evidence, err := agent.Verify(ctx, path, command, item.ProcessScope)
		out.Evidence = append(out.Evidence, evidence)
		return out, err
	})
}

func allowDispatch(ctx context.Context, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	requested, err := execution.CancelRequested(ctx, runID)
	if err != nil {
		return err
	}
	if requested {
		return hatchet.NewNonRetryableError(execution.ErrCancelled)
	}
	return nil
}
