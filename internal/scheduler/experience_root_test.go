package scheduler

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func TestRootRejectsTransportChangesToAcceptedSnapshot(t *testing.T) {
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	good := runInput{RequestID: request.ID, Snapshot: request.Snapshot}
	if _, err := loadRootExperience(ctx, ns, run, good); err != nil {
		t.Fatal(err)
	}
	changes := []func(*runInput){
		func(in *runInput) { in.Workflow.Nodes = in.Workflow.Nodes[:1] },
		func(in *runInput) { in.SourceRunID = uuid.NewString() },
		func(in *runInput) { in.ApprovalOrigin = uuid.NewString() },
		func(in *runInput) { in.Fork = &workflow.Fork{Commit: "forged"} },
		func(in *runInput) { in.Purpose = "evaluation" },
		func(in *runInput) { in.EntryKey = "workbench:forged" },
	}
	for i, change := range changes {
		input := good
		change(&input)
		if _, err := loadRootExperience(ctx, ns, run, input); err == nil {
			t.Fatalf("transport mutation %d bypassed accepted snapshot", i)
		}
	}
}

func TestProtectedLayerPreparesRevisionInputsAndRejectsUnprovenOutputs(t *testing.T) {
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	runtime, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	d := request.Snapshot.Workflow
	input := d.NodeInput(d.Nodes[0], nil)
	input.ReviewFeedback = &workflow.ReviewFeedback{ApprovalID: uuid.NewString(), NodeID: "second", Round: 2, Reason: "revise"}
	inputs := []workflow.NodeInput{input}
	execute := func(actual []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		if actual[0].RequestID != request.ID {
			t.Fatal("revision bypassed frozen experience preparation")
		}
		prepared := actual[0]
		prepared.RunID, prepared.NodeRunID = run, uuid.NewString()
		out := workflow.NodeOutput{Text: "revised"}
		if err := runtime.Record(ctx, prepared, prepared, &out); err != nil {
			return nil, err
		}
		return []workflow.NodeOutput{out}, nil
	}
	outputs, err := runtime.Execute(ctx, inputs, execute)
	if err != nil || len(outputs) != 1 || outputs[0].Lineage == nil {
		t.Fatal(outputs, err)
	}
	if inputs[0].RequestID != request.ID {
		t.Fatal("root observer cannot see final dispatch input")
	}
	bad := func([]workflow.NodeInput) ([]workflow.NodeOutput, error) {
		return []workflow.NodeOutput{{Text: "unproven"}}, nil
	}
	if _, err = runtime.Execute(ctx, inputs, bad); err == nil {
		t.Fatal("revision accepted unproven result")
	}
}
