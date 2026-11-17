package approval

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestApprovalGenerationStartsIndependentHumanReviewRounds(t *testing.T) {
	actor, request, run, input, old := generationFixture(t, "changes_requested")
	ctx := context.Background()
	if _, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "upstream changed"); err != nil {
		t.Fatal(err)
	}
	if err := ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	fresh := generationInput(t, input, 2)
	fresh.ApprovalRound = 2
	if _, err := Request(ctx, actor.Namespace, request, run, fresh); !errors.Is(err, ErrNotFound) {
		t.Fatal("old generation's correction reused as new generation round one", err)
	}
	fresh.ApprovalRound = 1
	first, err := Request(ctx, actor.Namespace, request, run, fresh)
	if err != nil || first.Round != 1 || first.PreviousID != "" || first.ID == old.ID {
		t.Fatalf("new generation did not start a distinct review: %+v %v", first, err)
	}
	decision := Decision{SubjectHash: first.SubjectHash, Decision: "changes_requested", Reason: "fresh human correction"}
	if _, err = Decide(ctx, actor, first.ID, decision); err != nil {
		t.Fatal(err)
	}
	fresh.ApprovalRound = 2
	if _, err = Request(ctx, actor.Namespace, request, run, fresh); !errors.Is(err, ErrInvalid) {
		t.Fatal("fresh human correction was not enforced", err)
	}
	fresh = resolveGenerationReview(t, fresh)
	second, err := Request(ctx, actor.Namespace, request, run, fresh)
	if err != nil || second.PreviousID != first.ID || second.Round != 2 || second.ApprovalGeneration != 2 {
		t.Fatalf("review linked to the wrong generation: %+v %v", second, err)
	}
	page, err := Browse(ctx, actor.Namespace, Query{ProjectID: input.ProjectID, Limit: 20})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != second.ID {
		t.Fatal("latest generation round not selected", err)
	}
}

func resolveGenerationReview(t *testing.T, input workflow.NodeInput) workflow.NodeInput {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal([]byte(input.Parents["plan"].Text), &report); err != nil {
		t.Fatal(err)
	}
	report["review"].(map[string]any)["resolutions"] = []workflow.ReviewResolution{{ItemID: "overall", Status: "addressed", Explanation: "resolved fresh correction"}}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	input.Parents = map[string]workflow.NodeOutput{"plan": {Text: string(raw)}}
	return input
}
