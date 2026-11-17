package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (r *nodeExperience) Run(ctx context.Context, d workflow.Definition, reused map[string]workflow.NodeOutput, before func(*workflow.NodeInput, map[string]workflow.NodeOutput) error, execute func([]workflow.NodeInput) ([]workflow.NodeOutput, error)) (map[string]workflow.NodeOutput, error) {
	if len(reused) != 0 || r.request.Snapshot.Fork != nil {
		if err := r.ValidateReused(ctx, reused); err != nil {
			return nil, err
		}
	}
	prepare := func(input *workflow.NodeInput, outputs map[string]workflow.NodeOutput) error {
		if err := r.Prepare(ctx, input); err != nil {
			return err
		}
		if before != nil {
			return before(input, outputs)
		}
		return nil
	}
	observe := func(input workflow.NodeInput, out *workflow.NodeOutput) error { return r.Observe(ctx, input, out) }
	return workflow.RunAudited(d, reused, prepare, observe, execute)
}
