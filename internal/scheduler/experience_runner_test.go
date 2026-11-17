package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestExperiencedRunnerSelectsBeforeApprovalAndVerifiesEveryOutput(t *testing.T) {
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	runtime, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	approvals := 0
	before := func(input *workflow.NodeInput, _ map[string]workflow.NodeOutput) error {
		if input.RequestID != request.ID {
			t.Fatal("approval preceded frozen experience preparation")
		}
		approvals++
		return nil
	}
	execute := func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		outputs := make([]workflow.NodeOutput, len(inputs))
		for i, input := range inputs {
			input.RunID, input.NodeRunID = run, uuid.NewString()
			outputs[i] = workflow.NodeOutput{Text: "verified"}
			if err := runtime.Record(ctx, input, input, &outputs[i]); err != nil {
				return nil, err
			}
		}
		return outputs, nil
	}
	outputs, err := runtime.Run(ctx, request.Snapshot.Workflow, nil, before, execute)
	if err != nil {
		t.Fatal(err)
	}
	if approvals != 2 || outputs["second"].Lineage == nil {
		t.Fatalf("missing protected path: approvals=%d outputs=%+v", approvals, outputs)
	}
	bad := func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		return []workflow.NodeOutput{{RunID: uuid.NewString(), Text: "unproven"}}, nil
	}
	if _, err = runtime.Run(ctx, request.Snapshot.Workflow, nil, before, bad); err == nil {
		t.Fatal("runner accepted unproven worker result")
	}
}
