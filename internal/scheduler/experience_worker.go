package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func executeExperienced(ctx context.Context, namespace string, item execution.Attempt, input workflow.NodeInput, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	runtime, err := loadNodeExperience(ctx, namespace, input.RequestID, item.RunID, input.ProjectID)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	if err = runtime.checkDispatch(ctx, input); err != nil {
		return workflow.NodeOutput{}, err
	}
	input.RunID, input.NodeRunID = item.RunID, item.NodeRunID
	observe := func(actual workflow.NodeInput, out *workflow.NodeOutput) error {
		return runtime.Record(ctx, input, actual, out)
	}
	out, err := executeNodeObserved(ctx, item, input, execute, observe)
	if err == nil {
		err = runtime.Observe(ctx, input, &out)
	}
	return out, err
}

func (r *nodeExperience) checkDispatch(ctx context.Context, input workflow.NodeInput) error {
	if err := r.checkDecisions(ctx, input); err != nil {
		return err
	}
	expected := input
	if err := r.Prepare(ctx, &expected); err != nil {
		return err
	}
	got, err := workflow.EffectiveInputHash(input)
	if err != nil {
		return err
	}
	want, err := workflow.EffectiveInputHash(expected)
	if err != nil {
		return err
	}
	if got != want {
		return experience.ErrInvalidated
	}
	generation := input.ApprovalGeneration
	if generation == 0 {
		generation = 1
	}
	if err := approval.CheckGeneration(ctx, r.namespace, r.runID, input.Node.ID, generation); err != nil {
		return err
	}
	return nil
}
