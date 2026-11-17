package catalog

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestCandidateFlowCompilesActualRoleAndStepTask(t *testing.T) {
	base := Version{ID: uuid.NewString(), Kind: "agent", Spec: Spec{Agent: nodePointer(candidateBase())}}
	flow := Version{ID: uuid.NewString(), Kind: "workflow", Spec: Spec{Flow: &Flow{Nodes: []Binding{
		{ID: "check", AgentVersion: base.ID, Task: "只能读取，不能修改", ReadOnly: true},
	}}}}
	candidate := candidateBase()
	candidate.Instruction = "根据经验检查项目"
	payload := CandidatePayload{Agent: candidate, FlowVersion: flow.ID, BindingIDs: []string{"check"}}
	lookup := func(id string) (*Version, error) {
		if id == base.ID {
			return &base, nil
		}
		if id == flow.ID {
			return &flow, nil
		}
		return nil, ErrNotFound
	}
	out, err := prepareCandidate(context.Background(), uuid.NewString(), base.ID, payload, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if out.Candidate.Nodes[0].Instruction != "根据经验检查项目\n\n本步骤任务（在角色权限和交接契约内执行）：\n只能读取，不能修改" {
		t.Fatal("候选未编译真实角色+步骤任务", out.Candidate.Nodes[0].Instruction)
	}
	if out.Candidate.VersionID != flow.ID || out.Candidate.Nodes[0].AgentVersion != base.ID || !out.Candidate.Nodes[0].ReadOnly {
		t.Fatal("实验删除了固定版本引用或只读约束")
	}
	if base.Spec.Agent.Instruction != "检查项目" || flow.Spec.Flow.Nodes[0].AgentVersion != base.ID {
		t.Fatal("预编译污染基线")
	}
	if out.BaselineHash == out.CandidateHash || out.CandidateCompiledHash == out.BaselineCompiledHash {
		t.Fatal("hash没有覆盖候选指令")
	}
}

func nodePointer(node workflow.Node) *workflow.Node { return &node }

func TestCandidateFlowUpgradeKeepsCompleteFlowAndRejectsUnapprovedBinding(t *testing.T) {
	flow := &Flow{Editor: &FlowEditor{Nodes: map[string]NodeDisplay{"a": {Name: "说明", X: 1}}},
		Budget: &workflow.Budget{}, Parameters: &workflow.Shape{Type: "object"},
		Loops: []workflow.Loop{{ID: "repair", Nodes: []string{"a"}}},
		Nodes: []Binding{{ID: "a", AgentVersion: "old", Task: "任务", ReadOnly: true,
			Approval: &workflow.Approval{SourceNode: "gate"}, ReportSchema: &workflow.Shape{Type: "object"}}}}
	upgraded, err := UpgradeFlowReferences(flow, "old", "new", []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.Nodes[0].AgentVersion != "new" || flow.Nodes[0].AgentVersion != "old" {
		t.Fatal("未替换或修改了原始Flow")
	}
	upgraded.Nodes[0].AgentVersion = "old"
	if !sameJSON(flow, upgraded) {
		t.Fatal("完整Flow字段在升级时丢失")
	}
	upgraded.Editor.Nodes["a"] = NodeDisplay{Name: "changed"}
	if flow.Editor.Nodes["a"].Name != "说明" {
		t.Fatal("Flow副本共享可变编辑信息")
	}
	for _, ids := range [][]string{nil, {"missing"}, {"a", "a"}} {
		if _, err := UpgradeFlowReferences(flow, "old", "new", ids); err == nil {
			t.Fatal("未授权或重复binding被接受", ids)
		}
	}
	if _, err := UpgradeFlowReferences(flow, "wrong", "new", []string{"a"}); err == nil {
		t.Fatal("基线引用漂移未拒绝")
	}
}
