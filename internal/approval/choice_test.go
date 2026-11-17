package approval

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func choiceReview(t *testing.T, mode string) *workflow.Review {
	t.Helper()
	var r workflow.Review
	raw := `{"title":"实施范围","summary":"确认方案","resolutions":[],"items":[{"id":"scope","category":"范围","title":"交付范围","proposal":"仅文档","rationale":"先验证","evidence_refs":[],"task_ids":[],"selection_mode":"` + mode + `","options":[{"id":"docs","label":"文档","description":"低投入"},{"id":"code","label":"代码","description":"增加开发投入"}],"recommended_option_ids":["docs"],"planned_option_ids":["docs"],"recommendation_reason":"先验证方向"}]}`
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestMultipleChoiceDecisionIsPersistentAndOrderIndependent(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	input := structuredInput(project, "文档")
	review := choiceReview(t, "multiple")
	body, _ := json.Marshal(map[string]any{"review": review})
	input.Parents["plan"] = workflow.NodeOutput{Text: string(body)}
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	d := choiceDecision(t, "changes_requested", "revise", `["docs","code"]`, "")
	d.SubjectHash = gate.SubjectHash
	if _, err = Decide(ctx, actor, gate.ID, d); err != nil {
		t.Fatal(err)
	}
	d.Items[0].SelectedOptionIDs = []string{"code", "docs"}
	if _, err = Decide(ctx, actor, gate.ID, d); err != nil {
		t.Fatal("多选顺序改变导致幂等冲突", err)
	}
	stored, err := Get(ctx, actor.Namespace, gate.ID)
	if err != nil || stored.Review.Items[0].RecommendationReason != "先验证方向" || len(stored.Items[0].SelectedOptionIDs) != 2 {
		t.Fatal("选择或推荐记录丢失", err)
	}
	if err = Consume(ctx, run, input, gate.ID); err == nil {
		t.Fatal("方案纠偏被误当成执行批准")
	}
}

func choiceDecision(t *testing.T, decision, itemDecision, ids, correction string) Decision {
	t.Helper()
	var d Decision
	raw := `{"decision":"` + decision + `","items":[{"item_id":"scope","decision":"` + itemDecision + `","correction":"` + correction + `","selected_option_ids":` + ids + `}]}`
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestChoiceApprovalCannotBypassPlanRevision(t *testing.T) {
	r := choiceReview(t, "single")
	for _, tt := range []struct {
		decision, answer, ids, correction string
		valid                             bool
	}{
		{"approved", "accept", `["docs"]`, "", true},
		{"approved", "accept", `["code"]`, "", false},
		{"approved", "accept", `[]`, "", false},
		{"approved", "accept", `["missing"]`, "", false},
		{"approved", "accept", `["docs","docs"]`, "", false},
		{"changes_requested", "revise", `["code"]`, "", true},
		{"changes_requested", "revise", `["docs"]`, "", false},
		{"changes_requested", "revise", `["docs","code"]`, "", false},
		{"changes_requested", "revise", `[]`, "需要另一个方案", true},
	} {
		d := choiceDecision(t, tt.decision, tt.answer, tt.ids, tt.correction)
		if err := validateAnswers(r, d); (err == nil) != tt.valid {
			t.Fatalf("%+v: %v", tt, err)
		}
	}
}
