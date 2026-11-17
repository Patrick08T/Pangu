package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestRootObserverRejectsLegacyResultsAndSealsSkippedDependency(t *testing.T) {
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
	old := workflow.NodeOutput{RunID: uuid.NewString(), Text: "unproven"}
	if err = runtime.Observe(ctx, input, &old); err == nil {
		t.Fatal("root accepted result without worker proof")
	}
	skipped := workflow.NodeOutput{Skipped: true, Text: "condition not matched"}
	if err = runtime.Observe(ctx, input, &skipped); err != nil {
		t.Fatal(err)
	}
	if skipped.Lineage == nil || skipped.RunID == "" {
		t.Fatal("skipped result bypassed dependency proof")
	}
	child := request.Snapshot.Workflow.NodeInput(request.Snapshot.Workflow.Nodes[1], map[string]workflow.NodeOutput{"first": skipped})
	if err = runtime.Prepare(ctx, &child); err != nil {
		t.Fatal(err)
	}
}
