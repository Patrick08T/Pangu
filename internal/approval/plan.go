package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type subject struct {
	Snapshot      json.RawMessage    `json:"snapshot"`
	Input         workflow.NodeInput `json:"node_input"`
	Head          string             `json:"head_commit"`
	Review        *workflow.Review   `json:"review"`
	Execution     *catalog.Execution `json:"execution,omitempty"`
	InheritedFrom string             `json:"inherited_from,omitempty"`
}

func approvalSubject(ctx context.Context, tx pgx.Tx, namespace, run string, snapshot json.RawMessage, input workflow.NodeInput, head string, review *workflow.Review) ([]byte, *Gate, error) {
	s := subject{Snapshot: snapshot, Input: input, Head: head, Review: review}
	inherited, err := inheritedGate(ctx, tx, namespace, run, snapshot, input, head)
	if err != nil {
		return nil, nil, err
	}
	if input.Node.Approval.ExecutionPlan {
		var original struct {
			Workflow workflow.Definition `json:"workflow"`
		}
		if err := json.Unmarshal(snapshot, &original); err != nil {
			return nil, nil, err
		}
		if inherited != nil {
			copy := *inherited.Execution
			copy.Workflow, copy.BaseVersion, copy.Changes = original.Workflow, original.Workflow.VersionID, []catalog.GraphChange{}
			s.Execution = &copy
		} else {
			p, parseErr := workflow.ParseExecutionPlan(input.Parents[input.Node.Approval.SourceNode].Text)
			if parseErr != nil {
				return nil, nil, parseErr
			}
			s.Execution, err = catalog.PrepareExecution(ctx, tx, namespace, original.Workflow, input.Node.ID, *p)
			if err != nil {
				return nil, nil, err
			}
		}
		if err = addGraphReview(review, s.Execution); err != nil {
			return nil, nil, err
		}
	}
	if inherited != nil {
		s.InheritedFrom = inherited.ID
	}
	body, err := json.Marshal(s)
	return body, inherited, err
}

func addGraphReview(review *workflow.Review, p *catalog.Execution) error {
	for _, item := range review.Items {
		if item.ID == "platform-execution-graph" {
			return fmt.Errorf("platform-execution-graph 是平台保留审核项")
		}
	}
	proposal := "任务已映射到实际执行节点，执行图无需变化。"
	if len(p.Changes) > 0 {
		proposal = fmt.Sprintf("执行图包含 %d 项变更。批准后发布新版本，保留已完成规划和报告，后续节点按新图执行；原运行与版本不改写。", len(p.Changes))
	}
	review.Items = append(review.Items, workflow.ReviewItem{ID: "platform-execution-graph", Category: "执行编排", Title: "任务映射与执行图", Proposal: proposal, Rationale: "由平台从冻结版本计算差异并校验，不以报告中的口头安排代替执行图。", EvidenceRefs: []string{}, TaskIDs: []string{}, SelectionMode: "single", Options: []workflow.ReviewOption{{ID: "apply", Label: "按此计划继续（推荐）", Description: "确认所列任务映射和执行图，存在变更时发布新版本续跑。"}, {ID: "revise", Label: "调整执行计划", Description: "填写纠偏意见，修订后重新审批。"}}, RecommendedOptionIDs: []string{"apply"}, PlannedOptionIDs: []string{"apply"}, RecommendationReason: "执行图已通过依赖与版本校验；业务取舍仍由你确认。"})
	return review.Validate()
}
