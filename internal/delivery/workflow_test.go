package delivery

import (
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func TestDeliveryPreservesSelectedScopeAndRequiresReadOnlyReview(t *testing.T) {
	project := uuid.NewString()
	implementation := catalog.Version{ID: uuid.NewString(), Kind: "agent", Spec: catalog.Spec{Agent: &workflow.Node{ID: "impl", Executor: "codex", Instruction: "按任务范围实施"}}}
	review := catalog.Version{ID: uuid.NewString(), Kind: "agent", Spec: catalog.Spec{Agent: &workflow.Node{ID: "review", Executor: "codex", Instruction: "只读检查", ReadOnly: true}}}
	report := research.Detail{ID: uuid.NewString(), SubjectHash: "digest", Data: &research.StructuredReport{Mode: "design", Tasks: []research.Task{{ID: "api", Title: "接口", Scope: "参数检查", Deliverable: "API", Acceptance: "错误参数返回 400"}, {ID: "ui", Title: "界面", Scope: "表单", DependsOn: []string{"api"}, Deliverable: "页面", Acceptance: "显示错误"}}}}
	input := CreateInput{SubjectHash: report.SubjectHash, TaskIDs: []string{"ui"}}
	if _, err := Build(uuid.NewString(), uuid.NewString(), project, input, report, implementation, review); err == nil {
		t.Fatal("选中任务缺少前置仍可执行")
	}
	input.TaskIDs = []string{"api", "ui"}
	d, err := Build(uuid.NewString(), uuid.NewString(), project, input, report, implementation, review)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Nodes) != 2 || d.Nodes[0].Approval == nil || !d.Nodes[1].ReadOnly || len(d.Nodes[1].DependsOn) != 1 {
		t.Fatalf("审批/只读审查未闭环: %+v", d)
	}
	var payload struct {
		Tasks []research.Task `json:"tasks"`
	}
	if err = json.Unmarshal([]byte(d.Input), &payload); err != nil || len(payload.Tasks) != 2 || payload.Tasks[1].Acceptance != "显示错误" {
		t.Fatalf("范围与验收条件丢失: %s", d.Input)
	}
	review.Spec.Agent.ReadOnly = false
	if _, err = Build(uuid.NewString(), uuid.NewString(), project, input, report, implementation, review); err == nil {
		t.Fatal("审查节点可写工作区")
	}
}
