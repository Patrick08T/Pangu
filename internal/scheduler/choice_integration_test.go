package scheduler_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func optionReport(revised bool) string {
	plan := "docs"
	resolutions := []workflow.ReviewResolution{}
	if revised {
		plan = "code"
		resolutions = append(resolutions, workflow.ReviewResolution{ItemID: "scope", Status: "addressed", Explanation: "已按所选代码方案修订范围与任务"})
	}
	r := workflow.Review{Title: "实施方案", Summary: "选择后必要时先修订", Resolutions: resolutions, Items: []workflow.ReviewItem{{ID: "scope", Category: "范围", Title: "交付方案", Proposal: plan, Rationale: "按投入取舍", EvidenceRefs: []string{}, TaskIDs: []string{}, SelectionMode: "single", Options: []workflow.ReviewOption{{ID: "docs", Label: "仅文档", Description: "低投入"}, {ID: "code", Label: "代码功能", Description: "需开发验证"}}, RecommendedOptionIDs: []string{"docs"}, PlannedOptionIDs: []string{plan}, RecommendationReason: "先验证方向"}}}
	body, _ := json.Marshal(map[string]any{"review": r})
	return string(body)
}

func TestHatchetChoiceSelectionRevisesPlanBeforeExecution(t *testing.T) {
	_, client := integrationClient(t)
	b := scheduler.NewBackend(client)
	p := registerTestProject(t, b)
	actor := approvalActor(t, b.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	var plans, builds atomic.Int32
	stop := startReviewWorker(t, client, func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		if input.Node.ID == "build" {
			builds.Add(1)
			r, err := workflow.ParseReview(input.Parents["plan"].Text)
			if err != nil || r.Items[0].PlannedOptionIDs[0] != "code" {
				return workflow.NodeOutput{}, errors.New("执行收到未按选择修订的旧计划")
			}
			return workflow.NodeOutput{Text: "完成"}, nil
		}
		plans.Add(1)
		if f := input.ReviewFeedback; f != nil {
			if len(f.Items) != 1 || len(f.Items[0].SelectedOptionIDs) != 1 || f.Items[0].SelectedOptionIDs[0] != "code" || f.PreviousReview.Items[0].Options[1].Label != "代码功能" {
				return workflow.NodeOutput{}, errors.New("选项或语义丢失")
			}
		}
		return workflow.NodeOutput{Text: optionReport(input.ReviewFeedback != nil)}, nil
	})
	defer stop()
	r, err := b.Submit(ctx, approvalWorkflow(p.ID, "test"), "choice-review")
	if err != nil {
		t.Fatal(err)
	}
	first := awaitReviewRound(t, b, r.RunID, 1)
	decision := approval.Decision{SubjectHash: first.SubjectHash, Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "revise", SelectedOptionIDs: []string{"code"}}}}
	if _, err = approval.Decide(ctx, actor, first.ID, decision); err != nil {
		t.Fatal(err)
	}
	if err = b.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	second := awaitReviewRound(t, b, r.RunID, 2)
	if builds.Load() != 0 || plans.Load() != 2 || second.Review.Items[0].PlannedOptionIDs[0] != "code" {
		t.Fatal("更换方案后未先修订复审")
	}
	decision = approval.Decision{SubjectHash: second.SubjectHash, Decision: "approved", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept", SelectedOptionIDs: []string{"code"}}}}
	if _, err = approval.Decide(ctx, actor, second.ID, decision); err != nil {
		t.Fatal(err)
	}
	if err = b.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	waitRun(t, api.New(b, integrationAccess{}), r.RunID)
	if builds.Load() != 1 {
		t.Fatal("执行次数错误")
	}
}
