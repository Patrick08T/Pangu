package workflow

func ReviewShape() Shape {
	s := Shape{Type: "string"}
	list := Shape{Type: "array", Items: &s}
	item := Shape{Type: "object", Properties: map[string]Shape{"id": s, "category": s, "title": s, "proposal": s, "rationale": s, "evidence_refs": list, "task_ids": list}, Required: []string{"id", "category", "title", "proposal", "rationale", "evidence_refs", "task_ids"}}
	option := Shape{Type: "object", Properties: map[string]Shape{"id": s, "label": s, "description": s}, Required: []string{"id", "label", "description"}}
	item.Properties["selection_mode"], item.Properties["recommendation_reason"] = s, s
	item.Properties["recommended_option_ids"], item.Properties["planned_option_ids"] = list, list
	item.Properties["options"] = Shape{Type: "array", Items: &option}
	resolution := Shape{Type: "object", Properties: map[string]Shape{"item_id": s, "status": s, "explanation": s}, Required: []string{"item_id", "status", "explanation"}}
	return Shape{Type: "object", Properties: map[string]Shape{"title": s, "summary": s, "items": {Type: "array", Items: &item}, "resolutions": {Type: "array", Items: &resolution}}, Required: []string{"title", "summary", "items", "resolutions"}}
}
