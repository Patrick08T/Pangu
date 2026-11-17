package delivery

import "github.com/Patrick08T/Pangu/internal/workflow"

func approvalPlan(reviewer workflow.Node) workflow.Node {
	s := workflow.Shape{Type: "string"}
	strings := workflow.Shape{Type: "array", Items: &s}
	task := workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"id": s, "title": s, "scope": s, "depends_on": strings, "deliverable": s, "acceptance": s}, Required: []string{"id", "title", "scope", "depends_on", "deliverable", "acceptance"}}
	summary := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"summary": s}, Required: []string{"summary"}}
	report := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"summary": s, "tasks": {Type: "array", Items: &task}, "review": workflow.ReviewShape()}, Required: []string{"summary", "tasks", "review"}}
	return workflow.Node{ID: "delivery_plan", Executor: "codex", ReadOnly: true, Model: reviewer.Model, Handoff: reviewer.Handoff, OutputSchema: summary, ReportSchema: report,
		Instruction: "只读核对 input 中已选择的任务，输出完整 tasks 和结构化 review，逐项列出范围、每项任务与验收供人工决定。review 每项含稳定 id、category、title、proposal、rationale、evidence_refs（本报告内 JSON Pointer）、task_ids（引用本报告 tasks.id）。首轮 resolutions=[]。纠偏时按 review_feedback 修订任务与审核项，resolutions 逐项回应，整体意见用 overall。不得实施、扩大文件权限、降低安全要求或声称已批准。最终输出严格满足报告结构。" + workflow.ReviewChoiceInstruction}
}
