package scheduler

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func dispatchExperienced(ctx context.Context, namespace, run string, input *workflow.NodeInput, dispatch func() error) error {
	runtime, err := loadNodeExperience(ctx, namespace, input.RequestID, run, input.ProjectID)
	if err != nil {
		return err
	}
	if err = runtime.checkDecisions(ctx, *input); err != nil {
		return err
	}
	if err = runtime.Prepare(ctx, input); err != nil {
		return err
	}
	if err = runtime.checkDispatch(ctx, *input); err != nil {
		return err
	}
	return dispatch()
}

func finishDispatchedLayer(inputs []workflow.NodeInput, jobs []*nodeDispatch, finish func() error) error {
	err := finish()
	for _, job := range jobs {
		inputs[job.index] = job.input
	}
	return err
}
