package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestNodeExecutionSealsFinalOutputBeforeReturning(t *testing.T) {
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	runtime, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	input := request.Snapshot.Workflow.NodeInput(request.Snapshot.Workflow.Nodes[0], nil)
	if err = runtime.Prepare(ctx, &input); err != nil {
		t.Fatal(err)
	}
	item := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: input.Node.ID, Attempt: 1}
	prepared := input
	prepared.RunID, prepared.NodeRunID = run, item.NodeRunID
	called := 0
	out, err := executeNodeObserved(ctx, item, input, func(_ context.Context, actual workflow.NodeInput) (workflow.NodeOutput, error) {
		called++
		return workflow.NodeOutput{Text: "actual result"}, nil
	}, func(actual workflow.NodeInput, out *workflow.NodeOutput) error {
		return runtime.Record(ctx, prepared, actual, out)
	})
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 || out.Lineage == nil || out.Lineage.ExecutionInputHash == "" {
		t.Fatalf("execution escaped proof: calls=%d out=%+v", called, out)
	}
	rejected := errors.New("proof rejected")
	_, err = executeNodeObserved(ctx, item, input, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "result"}, nil
	}, func(workflow.NodeInput, *workflow.NodeOutput) error { return rejected })
	if !errors.Is(err, rejected) {
		t.Fatalf("observer failure ignored: %v", err)
	}
}
