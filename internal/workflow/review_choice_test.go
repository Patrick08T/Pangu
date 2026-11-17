package workflow

import (
	"encoding/json"
	"testing"
)

func choiceReport(t *testing.T) map[string]any {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal([]byte(reviewReport(t)), &report); err != nil {
		t.Fatal(err)
	}
	item := report["review"].(map[string]any)["items"].([]any)[0].(map[string]any)
	item["selection_mode"] = "single"
	item["options"] = []any{map[string]any{"id": "docs", "label": "先交付文档", "description": "快速核验方向，暂不改变产品行为"}, map[string]any{"id": "code", "label": "实现产品功能", "description": "直接验证使用价值，需要增加开发和验证范围"}}
	item["recommended_option_ids"] = []string{"docs"}
	item["planned_option_ids"] = []string{"docs"}
	item["recommendation_reason"] = "先验证方向，控制本轮投入"
	return report
}

func TestReviewAcceptsChoicesAndRejectsInvalidRecommendation(t *testing.T) {
	report := choiceReport(t)
	raw, _ := json.Marshal(report)
	if _, err := ParseReview(string(raw)); err != nil {
		t.Fatal("结构化方案无法进入审核", err)
	}
	for _, kind := range []string{"mode", "one_option", "duplicate", "unknown_recommended", "no_recommended", "unknown_planned", "multi_planned_single", "reason"} {
		t.Run(kind, func(t *testing.T) {
			r := choiceReport(t)
			i := r["review"].(map[string]any)["items"].([]any)[0].(map[string]any)
			switch kind {
			case "mode":
				i["selection_mode"] = "automatic"
			case "one_option":
				i["options"] = i["options"].([]any)[:1]
			case "duplicate":
				i["options"].([]any)[1] = i["options"].([]any)[0]
			case "unknown_recommended":
				i["recommended_option_ids"] = []string{"missing"}
			case "no_recommended":
				i["recommended_option_ids"] = []string{}
			case "unknown_planned":
				i["planned_option_ids"] = []string{"missing"}
			case "multi_planned_single":
				i["planned_option_ids"] = []string{"docs", "code"}
			case "reason":
				i["recommendation_reason"] = ""
			}
			raw, _ := json.Marshal(r)
			if _, err := ParseReview(string(raw)); err == nil {
				t.Fatal("接受无效方案集")
			}
		})
	}
}

func TestChoiceReviewShapeFitsReportDepthAndPreservesOptions(t *testing.T) {
	schema := Shape{Type: "object", Properties: map[string]Shape{"review": ReviewShape()}, Required: []string{"review"}}
	compiled, err := schema.Compile()
	if err != nil {
		t.Fatal("选项结构超出平台约束", err)
	}
	report := choiceReport(t)
	normalized, _ := json.Marshal(map[string]any{"review": report["review"]})
	var value any
	if err = json.Unmarshal(normalized, &value); err != nil {
		t.Fatal(err)
	}
	if err = compiled.Validate(value); err != nil {
		t.Fatal("报告结构拒绝合法方案", err)
	}
	item := report["review"].(map[string]any)["items"].([]any)[0].(map[string]any)
	item["selection_mode"] = "multiple"
	item["recommended_option_ids"] = []string{"docs", "code"}
	item["planned_option_ids"] = []string{}
	raw, _ := json.Marshal(report)
	if _, err = ParseReview(string(raw)); err != nil {
		t.Fatal("尚未确定的组合决策被拒绝", err)
	}
}
