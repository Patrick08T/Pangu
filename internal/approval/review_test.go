package approval

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func structuredInput(project, proposal string) workflow.NodeInput {
	review := workflow.Review{Title: "范围审核", Summary: "请逐项决定", Items: []workflow.ReviewItem{{ID: "scope", Category: "范围", Title: "实施范围", Proposal: proposal, Rationale: "最小交付", TaskIDs: []string{}, EvidenceRefs: []string{}}}, Resolutions: []workflow.ReviewResolution{}}
	body, _ := json.Marshal(map[string]any{"review": review})
	return workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}, Parents: map[string]workflow.NodeOutput{"plan": {Text: string(body)}}}
}

func withReview(body string) string {
	var value, source map[string]any
	_ = json.Unmarshal([]byte(body), &value)
	_ = json.Unmarshal([]byte(structuredInput("", "原范围").Parents["plan"].Text), &source)
	value["review"] = source["review"]
	raw, _ := json.Marshal(value)
	return string(raw)
}

func acceptedItems() []workflow.ReviewAnswer {
	return []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}}
}

func TestApprovalRevisionKeepsHistoryAndRequiresNewDecision(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	input := structuredInput(project, "原建议")
	first, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	correction := Decision{SubjectHash: first.SubjectHash, Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "revise", Correction: "仅交付文档"}}}
	if _, err = Decide(ctx, actor, first.ID, correction); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, first.ID); err == nil {
		t.Fatal("纠偏意见被当成批准")
	}
	revised := structuredInput(project, "仅交付文档")
	revised.ApprovalRound = 2
	if _, err = Request(ctx, actor.Namespace, request, run, revised); err == nil {
		t.Fatal("遗漏纠偏处理仍创建复审")
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(revised.Parents["plan"].Text), &body)
	body["review"].(map[string]any)["resolutions"] = []workflow.ReviewResolution{{ItemID: "scope", Status: "addressed", Explanation: "已收窄为文档"}}
	raw, _ := json.Marshal(body)
	revised.Parents["plan"] = workflow.NodeOutput{Text: string(raw)}
	second, err := Request(ctx, actor.Namespace, request, run, revised)
	if err != nil || second.Round != 2 || second.PreviousID != first.ID || second.Status != "waiting" {
		t.Fatalf("复审轮次不正确: %+v %v", second, err)
	}
	repeated, err := Request(ctx, actor.Namespace, request, run, revised)
	if err != nil || repeated.ID != second.ID {
		t.Fatal("重试重复创建轮次", err)
	}
	decision := Decision{SubjectHash: second.SubjectHash, Decision: "approved", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}}}
	if _, err = Decide(ctx, actor, second.ID, decision); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, revised, second.ID); err != nil {
		t.Fatal(err)
	}
	old, err := Get(ctx, actor.Namespace, first.ID)
	if err != nil || old.Status != "changes_requested" || old.Items[0].Correction != "仅交付文档" {
		t.Fatal("旧意见丢失", err)
	}
}

func TestItemDecisionCannotApproveCorrectionsOrMissingItems(t *testing.T) {
	review := &workflow.Review{Items: []workflow.ReviewItem{{ID: "direction"}, {ID: "scope"}}}
	accept := workflow.ReviewAnswer{ItemID: "direction", Decision: "accept"}
	cases := []Decision{
		{Decision: "approved", Items: []workflow.ReviewAnswer{accept}},
		{Decision: "approved", Items: []workflow.ReviewAnswer{accept, accept}},
		{Decision: "approved", Items: []workflow.ReviewAnswer{accept, {ItemID: "scope", Decision: "accept", Correction: "扩大范围"}}},
		{Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "missing", Decision: "revise", Correction: "修改"}}},
		{Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "revise"}}},
	}
	for _, d := range cases {
		if validateAnswers(review, d) == nil {
			t.Fatalf("接受无效决定: %+v", d)
		}
	}
	good := Decision{Decision: "approved", Items: []workflow.ReviewAnswer{accept, {ItemID: "scope", Decision: "accept"}}}
	if err := validateAnswers(review, good); err != nil {
		t.Fatal(err)
	}
	review.Resolutions = []workflow.ReviewResolution{{ItemID: "scope", Status: "unresolved", Explanation: "待处理"}}
	if validateAnswers(review, good) == nil {
		t.Fatal("未处理纠偏仍批准")
	}
	if err := validateAnswers(review, Decision{Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "revise", Correction: "请收窄"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalCorrectionRequiresResponseAndRepeatedDecisionsAreStable(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	input := structuredInput(project, "原范围")
	first, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	d := Decision{SubjectHash: first.SubjectHash, Decision: "changes_requested", Reason: "整体收窄为文档"}
	for range 2 {
		if _, err = Decide(ctx, actor, first.ID, d); err != nil {
			t.Fatal("相同决定重试失败", err)
		}
	}
	d.Reason = "改为扩展实施"
	if _, err = Decide(ctx, actor, first.ID, d); err == nil {
		t.Fatal("同一轮决定被二次覆盖")
	}
	input.ApprovalRound = 2
	if _, err = Request(ctx, actor.Namespace, request, run, input); err == nil {
		t.Fatal("未回应整体纠偏仍提交复审")
	}
	var report map[string]any
	_ = json.Unmarshal([]byte(input.Parents["plan"].Text), &report)
	report["review"].(map[string]any)["resolutions"] = []workflow.ReviewResolution{{ItemID: "overall", Status: "unresolved", Explanation: "需要补充资料"}}
	raw, _ := json.Marshal(report)
	input.Parents["plan"] = workflow.NodeOutput{Text: string(raw)}
	second, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Decide(ctx, actor, second.ID, Decision{SubjectHash: second.SubjectHash, Decision: "approved", Items: acceptedItems()}); err == nil {
		t.Fatal("整体纠偏未解决仍批准")
	}
	page, err := Browse(ctx, actor.Namespace, Query{ProjectID: project, Limit: 20})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != second.ID {
		t.Fatal("审核中心未聚合最新轮次", err)
	}
	history, err := Browse(ctx, actor.Namespace, Query{ProjectID: project, RunID: run, Limit: 1})
	if err != nil || len(history.Items) != 1 || history.NextOffset == nil {
		t.Fatal("历史轮次未分页保留", err)
	}
}
