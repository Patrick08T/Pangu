package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func enrichModelRetries(ctx context.Context, run *RunDetails) error {
	if terminal(run.Status) || run.CancelRequested {
		return nil
	}
	seen := map[string]bool{}
	for i := range run.Nodes {
		node := &run.Nodes[i]
		if !currentLoopRound(run, *node) {
			continue
		}
		if seen[node.NodeID] {
			continue
		}
		seen[node.NodeID] = true
		if node.Status != "failed" || node.Error == nil || !workflow.ModelRetryCategory(node.Error.Category) {
			continue
		}
		at, err := execution.ModelRetryAt(ctx, run.ID, node.ID, node.ModelRetry)
		if err != nil {
			return err
		}
		node.NextRetryAt = at
	}
	return nil
}

type nodeDispatch struct {
	input           workflow.NodeInput
	task            *hatchet.StandaloneTask
	name, namespace string
	ref             *hatchet.WorkflowRunRef
	index, retry    int
	at              *time.Time
}

func dispatchNode(ctx hatchet.DurableContext, job *nodeDispatch) error {
	if err := waitNode(ctx.GetContext(), ctx.WorkflowRunId(), job.input.Node.ID); err != nil {
		return err
	}
	return dispatchExperienced(ctx.GetContext(), job.namespace, ctx.WorkflowRunId(), &job.input, func() error {
		key, metadata := nodeRunIdentity(job.input)
		if job.retry > 0 {
			key = fmt.Sprintf("%s-model-retry-%d", key, job.retry)
			metadata["pangu_model_retry"] = fmt.Sprint(job.retry)
		}
		ref, err := job.task.RunNoWait(ctx, job.input, hatchet.WithRunKey(key), hatchet.WithRunMetadata(metadata))
		job.ref = ref
		return err
	})
}

func collectNode(ctx hatchet.DurableContext, job *nodeDispatch, outputs []workflow.NodeOutput) error {
	result, err := job.ref.Result()
	job.at = nil
	if err != nil {
		at, lookupErr := execution.ModelRetryAt(ctx.GetContext(), ctx.WorkflowRunId(), job.ref.RunId, job.retry)
		if lookupErr != nil {
			return errors.Join(err, lookupErr)
		}
		if at != nil {
			job.at = at
			return nil
		}
		return fmt.Errorf("节点 %s: %w", job.input.Node.ID, err)
	}
	var output workflow.NodeOutput
	if err := result.TaskOutput(job.name).Into(&output); err != nil {
		return err
	}
	output.RunID = job.ref.RunId
	outputs[job.index] = output
	return nil
}

func retryNodes(ctx hatchet.DurableContext, jobs []*nodeDispatch, outputs []workflow.NodeOutput) error {
	pending := []*nodeDispatch{}
	for _, job := range jobs {
		if job.at != nil {
			pending = append(pending, job)
		}
	}
	for len(pending) > 0 {
		// 只依赖持久结果排序，不能用当前时间决定派生顺序，否则重放会重复节点。
		sort.SliceStable(pending, func(i, j int) bool { return pending[i].at.Before(*pending[j].at) })
		job := pending[0]
		if err := allowDispatch(ctx.GetContext(), ctx.WorkflowRunId()); err != nil {
			return err
		}
		if _, err := ctx.SleepUntil(*job.at); err != nil {
			return err
		}
		job.retry++
		if err := dispatchNode(ctx, job); err != nil {
			return err
		}
		if err := collectNode(ctx, job, outputs); err != nil {
			return err
		}
		if job.at == nil {
			pending = pending[1:]
		}
	}
	return nil
}
