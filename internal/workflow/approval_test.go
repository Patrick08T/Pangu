package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApprovalWaitHasNoDeadline(t *testing.T) {
	n := Node{ID: "build", Executor: "codex", Instruction: "实施", Approval: &Approval{}}
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(n.Approval)
	if err != nil || strings.Contains(string(raw), "timeout") || strings.Contains(string(raw), "expires") {
		t.Fatalf("审批仍暴露等待期限: %s %v", raw, err)
	}
}

func TestApprovalRequiresExplicitDependencyForWorkspaceWriters(t *testing.T) {
	d := Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Nodes: []Node{{ID: "first", Executor: "codex", Instruction: "写入", DependsOn: []string{"plan"}, Approval: &Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}, {ID: "second", Executor: "codex", Instruction: "写入", DependsOn: []string{"plan"}}, {ID: "plan", Executor: "codex", ReadOnly: true, Instruction: "规划"}}}
	if _, err := d.Layers(); err == nil {
		t.Fatal("允许并列写入使批准检查点失效")
	}
	d.Nodes[0].ReadOnly, d.Nodes[1].ReadOnly = true, true
	if _, err := d.Layers(); err == nil {
		t.Fatal("允许并列只读 Codex 推进检查点使审批失效")
	}
	d.Nodes[1].DependsOn = []string{"first"}
	if _, err := d.Layers(); err != nil {
		t.Fatal(err)
	}
}
