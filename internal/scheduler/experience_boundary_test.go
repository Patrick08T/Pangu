package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestActualExecutionCannotChangeApprovedLogicalInput(t *testing.T) {
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
	changes := []func(*workflow.NodeInput){
		func(in *workflow.NodeInput) { in.Input = "substituted" },
		func(in *workflow.NodeInput) { in.Node.Instruction = "substituted" },
		func(in *workflow.NodeInput) { in.ApprovalRound++ },
		func(in *workflow.NodeInput) { in.RequestID = uuid.NewString() },
	}
	for i, change := range changes {
		input.RunID, input.NodeRunID = run, uuid.NewString()
		actual := input
		change(&actual)
		out := workflow.NodeOutput{Text: "result"}
		if err = runtime.Record(ctx, input, actual, &out); err == nil {
			t.Fatalf("mutation %d acquired valid proof", i)
		}
	}
}

func TestApprovalRebindKeepsLogicalAttemptAndDropsPriorSelection(t *testing.T) {
	d := workflow.Definition{Input: "task", Nodes: []workflow.Node{{ID: "n", Executor: "eino", Model: "fixture", Instruction: "task"}}}
	input := d.NodeInput(d.Nodes[0], nil)
	input.RequestID = "request"
	input.ApprovalGeneration = 3
	input.Iteration = &workflow.Iteration{ID: "loop", Round: 2}
	input.Experiences = []workflow.Experience{{Reference: workflow.MemoryReference{VersionID: "old"}}}
	got, err := rebindApprovalInput(d, input, nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestID != input.RequestID || got.ApprovalGeneration != 3 || got.Iteration == nil || *got.Iteration != *input.Iteration || got.ApprovalRound != 4 {
		t.Fatalf("lost logical context: %+v", got)
	}
	if len(got.Experiences) != 0 {
		t.Fatal("rebound review silently reused old selection")
	}
}
