package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func runAgent(ctx hatchet.Context, namespace string, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error), input workflow.NodeInput) (workflow.NodeOutput, error) {
	if err := input.Node.Validate(); err != nil {
		return workflow.NodeOutput{}, hatchet.NewNonRetryableError(err)
	}
	policy := input.Node.EffectivePolicy()
	nodeCtx := ctx.GetContext()
	parent := ctx.ParentWorkflowRunId()
	if parent == nil {
		return workflow.NodeOutput{}, hatchet.NewNonRetryableError(errors.New("Agent 节点必须由 Pangu 根运行派生"))
	}
	item := execution.Attempt{RunID: *parent, NodeRunID: ctx.WorkflowRunId(), NodeID: input.Node.ID, Attempt: ctx.RetryCount() + 1}
	item.ProcessScope = uuid.NewString()
	if err := beginAttempt(nodeCtx, &item); err != nil {
		return workflow.NodeOutput{}, hatchet.NewNonRetryableError(err)
	}
	out, err := tracedNode(nodeCtx, namespace, item, input, execute)
	if errors.Is(err, workflow.ErrStopUnconfirmed) {
		return out, hatchet.NewNonRetryableError(err)
	}
	item.Evidence, item.EvidenceTruncated = out.Evidence, out.EvidenceTruncated
	out.RunID, item.Status = ctx.WorkflowRunId(), "succeeded"
	var failure *workflow.Failure
	if nodeCtx.Err() != nil && !errors.As(err, &failure) {
		err = nodeCtx.Err()
	}
	retry := workflow.CanRetry(err) && !workflow.ModelRetryCategory(workflow.FailureKind(err)) && ctx.RetryCount() < policy.Retries
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

func tracedNode(ctx context.Context, namespace string, item execution.Attempt, input workflow.NodeInput, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	ctx = trace.WithRun(ctx, item.RunID, input.Node.ID, item.NodeRunID, item.Attempt)
	ctx, span, err := trace.Start(ctx, "node", input.Node.ID, input.Input)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	out, err := executeExperienced(ctx, namespace, item, input, execute)
	if traceErr := span.End(out.Text, err); traceErr != nil {
		return out, &workflow.Failure{Kind: "trace", Cause: errors.Join(err, traceErr)}
	}
	return out, err
}

func executeNode(ctx context.Context, item execution.Attempt, input workflow.NodeInput, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	return executeNodeObserved(ctx, item, input, execute, nil)
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
