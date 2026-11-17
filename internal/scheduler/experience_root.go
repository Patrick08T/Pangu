package scheduler

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func loadRootExperience(ctx context.Context, namespace, run string, input runInput) (*nodeExperience, error) {
	runtime, err := loadNodeExperience(ctx, namespace, input.RequestID, run, input.Workflow.ProjectID)
	if err != nil {
		return nil, err
	}
	accepted, err := json.Marshal(runtime.request.Snapshot)
	if err != nil {
		return nil, err
	}
	transported, err := json.Marshal(input.Snapshot)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(accepted, transported) {
		return nil, experience.ErrInvalidated
	}
	if err := runtime.ValidateApprovals(ctx); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Execute also protects revision layers that do not pass through runner.before.
// inputs remains the shared final dispatch slice used by the root observer.
func (r *nodeExperience) Execute(ctx context.Context, inputs []workflow.NodeInput, execute func([]workflow.NodeInput) ([]workflow.NodeOutput, error)) ([]workflow.NodeOutput, error) {
	for i := range inputs {
		if err := r.checkDecisions(ctx, inputs[i]); err != nil {
			return nil, err
		}
		if err := r.Prepare(ctx, &inputs[i]); err != nil {
			return nil, err
		}
		if err := r.checkDispatch(ctx, inputs[i]); err != nil {
			return nil, err
		}
	}
	outputs, err := execute(inputs)
	if err != nil {
		return nil, err
	}
	if len(outputs) != len(inputs) {
		return nil, experience.ErrInvalidated
	}
	for i := range outputs {
		if err := r.Observe(ctx, inputs[i], &outputs[i]); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}
