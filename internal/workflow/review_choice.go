package workflow

import (
	"fmt"
	"slices"
	"strings"
)

type ReviewOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

func (item ReviewItem) ValidateOptions() error {
	if len(item.Options) == 0 {
		if item.SelectionMode != "" || len(item.RecommendedOptionIDs) > 0 || len(item.PlannedOptionIDs) > 0 || item.RecommendationReason != "" {
			return fmt.Errorf("审核项 %s 缺少方案列表", item.ID)
		}
		return nil
	}
	if len(item.Options) < 2 || len(item.Options) > 6 || (item.SelectionMode != "single" && item.SelectionMode != "multiple") || !reviewText(item.RecommendationReason, 4000) {
		return fmt.Errorf("审核项 %s 的方案数量、选择方式或推荐理由无效", item.ID)
	}
	ids, labels := map[string]bool{}, map[string]bool{}
	for _, option := range item.Options {
		label := strings.TrimSpace(option.Label)
		if !identifier.MatchString(option.ID) || ids[option.ID] || labels[label] || !reviewText(label, 200) || !reviewText(option.Description, 4000) {
			return fmt.Errorf("审核项 %s 的方案标识、标题或说明无效", item.ID)
		}
		ids[option.ID], labels[label] = true, true
	}
	if len(item.RecommendedOptionIDs) == 0 || !item.ValidSelection(item.RecommendedOptionIDs) || !item.ValidSelection(item.PlannedOptionIDs) {
		return fmt.Errorf("审核项 %s 的推荐或当前计划引用无效", item.ID)
	}
	return nil
}

// ValidSelection 校验引用、重复项及互斥约束；空集允许表达尚未作出方案决定。
func (item ReviewItem) ValidSelection(ids []string) bool {
	if len(ids) > len(item.Options) || (item.SelectionMode == "single" && len(ids) > 1) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || !slices.ContainsFunc(item.Options, func(o ReviewOption) bool { return o.ID == id }) {
			return false
		}
		seen[id] = true
	}
	return true
}

func (item ReviewItem) MatchesPlan(ids []string) bool {
	if len(ids) != len(item.PlannedOptionIDs) {
		return false
	}
	for _, id := range ids {
		if !slices.Contains(item.PlannedOptionIDs, id) {
			return false
		}
	}
	return true
}

// ReviewChoiceInstruction 说明选择的含义，具体报告仍使用节点的冻结结构。
const ReviewChoiceInstruction = "审批或不确定事项存在多个可行方案时，在审核项中提供2至6个真实备选方案options（每项id、label、description说明收益及代价），互斥方案selection_mode=single，可组合方案=multiple；recommended_option_ids标注推荐组合，recommendation_reason给出依据、权衡和不确定性，planned_option_ids明确当前计划实际采用的组合，尚未确定则为空数组。推荐不等于批准，不默认代选；没有真实备选方案的审核项不制造选项。收到review_feedback.items.selected_option_ids时，结合previous_review同项方案解释落实用户选择并逐项回应；修改计划后同步planned_option_ids。保持可追踪的方案ID，不把用户选择直接当成实施批准。"
