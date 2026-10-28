package workflow

import "testing"

func TestApprovalHasSeparateBoundedWait(t *testing.T) {
	n := Node{ID: "build", Executor: "codex", Instruction: "实施", Approval: &Approval{TimeoutSeconds: 600}}
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, seconds := range []int{0, -1, 3601} {
		n.Approval.TimeoutSeconds = seconds
		if n.Validate() == nil {
			t.Fatalf("无界审批等待 %d", seconds)
		}
	}
}

func TestApprovalRequiresExplicitDependencyForWorkspaceWriters(t *testing.T) {
	d := Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Nodes: []Node{{ID: "first", Executor: "codex", Instruction: "写入", Approval: &Approval{TimeoutSeconds: 60}}, {ID: "second", Executor: "codex", Instruction: "写入"}}}
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
