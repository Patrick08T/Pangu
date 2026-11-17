package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Review struct {
	Title       string             `json:"title"`
	Summary     string             `json:"summary"`
	Items       []ReviewItem       `json:"items"`
	Resolutions []ReviewResolution `json:"resolutions"`
}
type ReviewItem struct {
	SelectionMode        string         `json:"selection_mode,omitempty"`
	Options              []ReviewOption `json:"options,omitempty"`
	RecommendedOptionIDs []string       `json:"recommended_option_ids,omitempty"`
	PlannedOptionIDs     []string       `json:"planned_option_ids,omitempty"`
	RecommendationReason string         `json:"recommendation_reason,omitempty"`
	ID                   string         `json:"id"`
	Category             string         `json:"category"`
	Title                string         `json:"title"`
	Proposal             string         `json:"proposal"`
	Rationale            string         `json:"rationale"`
	EvidenceRefs         []string       `json:"evidence_refs"`
	TaskIDs              []string       `json:"task_ids"`
}
type ReviewResolution struct {
	ItemID      string `json:"item_id"`
	Status      string `json:"status"`
	Explanation string `json:"explanation"`
}
type ReviewAnswer struct {
	SelectedOptionIDs []string `json:"selected_option_ids,omitempty"`
	ItemID            string   `json:"item_id"`
	Decision          string   `json:"decision"`
	Correction        string   `json:"correction"`
}
type ReviewFeedback struct {
	ApprovalID     string         `json:"approval_id"`
	NodeID         string         `json:"node_id"`
	Round          int            `json:"round"`
	Reason         string         `json:"reason"`
	Items          []ReviewAnswer `json:"items"`
	PreviousReview Review         `json:"previous_review"`
}

func ParseReview(body string) (*Review, error) {
	if len(body) > 1024*1024 {
		return nil, fmt.Errorf("审核报告超过 1 MiB")
	}
	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		return nil, fmt.Errorf("审核报告不是 JSON: %w", err)
	}
	var review Review
	decoder := json.NewDecoder(strings.NewReader(string(report["review"])))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&review); err != nil {
		return nil, fmt.Errorf("缺少或无效的 review: %w", err)
	}
	if err := review.Validate(); err != nil {
		return nil, err
	}
	var root any
	_ = json.Unmarshal([]byte(body), &root)
	var tasks []struct {
		ID string `json:"id"`
	}
	if raw := report["tasks"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &tasks); err != nil {
			return nil, err
		}
	}
	ids := map[string]bool{}
	for _, task := range tasks {
		ids[task.ID] = true
	}
	for _, item := range review.Items {
		for _, id := range item.TaskIDs {
			if !ids[id] {
				return nil, fmt.Errorf("审核项 %s 引用未知任务 %s", item.ID, id)
			}
		}
		for _, ref := range item.EvidenceRefs {
			if !strings.HasPrefix(ref, "#/") {
				return nil, fmt.Errorf("审核依据必须是报告内 JSON Pointer: %s", ref)
			}
			if _, err := selectField(root, strings.TrimPrefix(ref, "#")); err != nil {
				return nil, fmt.Errorf("审核项 %s 的依据不可定位: %w", item.ID, err)
			}
		}
	}
	return &review, nil
}

func (r Review) Validate() error {
	if !reviewText(r.Title, 200) || !reviewText(r.Summary, 4000) || len(r.Items) < 1 || len(r.Items) > 200 || r.Resolutions == nil {
		return fmt.Errorf("审核标题、摘要、items 或 resolutions 无效")
	}
	seen := map[string]bool{}
	for _, i := range r.Items {
		if !identifier.MatchString(i.ID) || i.ID == "overall" || seen[i.ID] || !reviewText(i.Category, 100) || !reviewText(i.Title, 200) || !reviewText(i.Proposal, 8000) || !reviewText(i.Rationale, 4000) || i.EvidenceRefs == nil || i.TaskIDs == nil {
			return fmt.Errorf("审核项 %s 字段缺失、重复或超长", i.ID)
		}
		seen[i.ID] = true
		if err := i.ValidateOptions(); err != nil {
			return err
		}
	}
	seen = map[string]bool{}
	for _, v := range r.Resolutions {
		if !identifier.MatchString(v.ItemID) || seen[v.ItemID] || (v.Status != "addressed" && v.Status != "unresolved") || !reviewText(v.Explanation, 4000) {
			return fmt.Errorf("纠偏处理记录 %s 无效", v.ItemID)
		}
		seen[v.ItemID] = true
	}
	return nil
}

func reviewText(s string, limit int) bool {
	return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= limit
}
