package workflow

import (
	"encoding/json"
	"fmt"
)

type EvidenceBlocker struct {
	ID                  string `json:"id"`
	Owner               string `json:"owner"`
	Reason              string `json:"reason"`
	ResolutionCondition string `json:"resolution_condition"`
}

type EvidenceRetry struct {
	ApprovalID string         `json:"approval_id"`
	Round      int            `json:"round"`
	Reason     string         `json:"reason"`
	Items      []ReviewAnswer `json:"items"`
}

func ParseEvidenceBlockers(body string) ([]EvidenceBlocker, error) {
	var report struct {
		Blockers []EvidenceBlocker `json:"evidence_blockers"`
	}
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		return nil, err
	}
	if err := ValidateEvidenceBlockers(report.Blockers); err != nil {
		return nil, err
	}
	return report.Blockers, nil
}

func ValidateEvidenceBlockers(items []EvidenceBlocker) error {
	if len(items) > 8 {
		return fmt.Errorf("最多列出 8 个关键补证阻塞")
	}
	seen := map[string]bool{}
	for _, b := range items {
		if !identifier.MatchString(b.ID) || b.ID == "overall" || seen[b.ID] || !reviewText(b.Owner, 100) || !reviewText(b.Reason, 600) || !reviewText(b.ResolutionCondition, 600) {
			return fmt.Errorf("阻塞 %s 必须有唯一 ID、负责人、原因与明确解除条件", b.ID)
		}
		seen[b.ID] = true
	}
	return nil
}

func EvidenceReview(items []EvidenceBlocker) Review {
	r := Review{Title: "补证与定向复验", Summary: "工作流已暂停。请按责任人与解除条件准备材料，选择复验，或填写资料说明后提交。此决定只触发同一只读节点重新核验，不表示验收通过，也不扩大实施范围。", Items: []ReviewItem{}, Resolutions: []ReviewResolution{}}
	for _, b := range items {
		r.Items = append(r.Items, ReviewItem{ID: b.ID, Category: "证据阻塞", Title: b.ID + " · " + b.Owner, Proposal: b.ResolutionCondition, Rationale: b.Reason, EvidenceRefs: []string{}, TaskIDs: []string{}, SelectionMode: "single", Options: []ReviewOption{{ID: "recheck", Label: "补证后复验（推荐）", Description: "资料已准备好，请原节点重新核验。"}, {ID: "clarify", Label: "纠正证据判断", Description: "填写已有证据位置或判断错误，重新核验；不能直接放行。"}}, RecommendedOptionIDs: []string{"recheck"}, PlannedOptionIDs: []string{"recheck"}, RecommendationReason: "保留其他节点成果，仅重新核验证据缺口。"})
	}
	return r
}

func EvidenceBlockersShape() Shape {
	s := Shape{Type: "string"}
	item := Shape{Type: "object", Properties: map[string]Shape{"id": s, "owner": s, "reason": s, "resolution_condition": s}, Required: []string{"id", "owner", "reason", "resolution_condition"}}
	return Shape{Type: "array", Items: &item}
}
