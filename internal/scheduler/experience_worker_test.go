package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestExperiencedWorkerRejectsUnboundReviewDecisions(t *testing.T) {
	for _, kind := range []string{"evidence", "revision"} {
		t.Run(kind, func(t *testing.T) {
			ns, request, run := runtimeExperienceFixture(t)
			ctx := context.Background()
			runtime, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
			if err != nil {
				t.Fatal(err)
			}
			input := request.Snapshot.Workflow.NodeInput(request.Snapshot.Workflow.Nodes[0], nil)
			if kind == "evidence" {
				input.EvidenceRetry = &workflow.EvidenceRetry{ApprovalID: uuid.NewString(), Round: 1}
			} else {
				input.ReviewFeedback = &workflow.ReviewFeedback{ApprovalID: uuid.NewString(), NodeID: "second", Round: 2}
			}
			if err = runtime.Prepare(ctx, &input); err != nil {
				t.Fatal(err)
			}
			calls := 0
			item := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: input.Node.ID, Attempt: 1}
			_, err = executeExperienced(ctx, ns, item, input, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
				calls++
				return workflow.NodeOutput{Text: "result"}, nil
			})
			if err == nil || calls != 0 {
				t.Fatalf("unbound %s decision reached executor: calls=%d err=%v", kind, calls, err)
			}
		})
	}
}

func TestExperiencedWorkerRejectsForgedSelectionBeforeExecutor(t *testing.T) {
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
	calls := 0
	execute := func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		calls++
		return workflow.NodeOutput{Text: "result"}, nil
	}
	forged := input
	forged.Experiences = []workflow.Experience{{Kind: "forged"}}
	if _, err = executeExperienced(ctx, ns, item, forged, execute); err == nil || calls != 0 {
		t.Fatalf("forged selection reached executor: calls=%d err=%v", calls, err)
	}
	out, err := executeExperienced(ctx, ns, item, input, execute)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || out.Lineage == nil {
		t.Fatalf("worker returned unsealed result: calls=%d out=%+v", calls, out)
	}
}
