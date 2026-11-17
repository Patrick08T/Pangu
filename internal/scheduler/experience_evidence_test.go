package scheduler

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func evidenceRuntimeFixture(t *testing.T) (*nodeExperience, workflow.NodeInput, workflow.NodeOutput) {
	t.Helper()
	return evidenceRuntimeGenerationFixture(t, 3)
}

func evidenceRuntimeGenerationFixture(t *testing.T, generation int) (*nodeExperience, workflow.NodeInput, workflow.NodeOutput) {
	t.Helper()
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	d := request.Snapshot.Workflow
	d.Nodes[0].ReadOnly, d.Nodes[0].Executor = true, "codex"
	request, err := submission.Prepare(ctx, ns, "submit", "evidence-runtime", submission.Snapshot{Workflow: d})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	input := d.NodeInput(d.Nodes[0], nil)
	input.ApprovalGeneration = generation
	input.Iteration = &workflow.Iteration{ID: "cycle", Round: 2}
	if err = runtime.Prepare(ctx, &input); err != nil {
		t.Fatal(err)
	}
	input.RunID, input.NodeRunID = run, uuid.NewString()
	out := workflow.NodeOutput{RunID: input.NodeRunID, Text: "needs evidence", Blockers: []workflow.EvidenceBlocker{{ID: "missing", Owner: "owner", Reason: "unverified", ResolutionCondition: "provide proof"}}}
	if err = runtime.Record(ctx, input, input, &out); err != nil {
		t.Fatal(err)
	}
	return runtime, input, out
}

func TestEvidenceApprovalRejectsUnprovenAndChangedOutput(t *testing.T) {
	runtime, input, out := evidenceRuntimeFixture(t)
	for _, change := range []func(*workflow.NodeOutput){func(out *workflow.NodeOutput) { out.Lineage = nil }, func(out *workflow.NodeOutput) { out.Text = "changed" }} {
		forged := out
		change(&forged)
		if _, err := runtime.EvidenceApprovalInput(context.Background(), input, forged); err == nil {
			t.Fatal("unproven source became human evidence gate")
		}
	}
}

func TestEvidenceApprovalPreservesGenerationAndOriginalSourceIdentity(t *testing.T) {
	runtime, input, out := evidenceRuntimeFixture(t)
	next, err := runtime.EvidenceApprovalInput(context.Background(), input, out)
	if err != nil {
		t.Fatal(err)
	}
	if next.RequestID != input.RequestID || next.ApprovalGeneration != input.ApprovalGeneration || next.Iteration == nil || *next.Iteration != *input.Iteration {
		t.Fatal("evidence gate lost logical attempt identity", next)
	}
	if next.Node.ID != "evidence-"+out.Lineage.ArtifactID {
		t.Fatal("evidence gate detached from original artifact", next.Node.ID)
	}
	parent := next.Parents[input.Node.ID]
	if parent.Lineage != nil {
		t.Fatal("transformed review falsely inherits original execution proof")
	}
	review, err := workflow.ParseReview(parent.Text)
	if err != nil || len(review.Items) != 1 {
		t.Fatal("missing blocker review", review, err)
	}
}
