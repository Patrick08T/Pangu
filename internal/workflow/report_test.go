package workflow

import "testing"

func TestReportSchemaRequiresReadonlyAndMatchingScalarSummary(t *testing.T) {
	summary := &Shape{Type: "object", Properties: map[string]Shape{"ready": {Type: "boolean"}}}
	full := &Shape{Type: "object", Properties: map[string]Shape{"ready": {Type: "boolean"}, "tasks": {Type: "array", Items: &Shape{Type: "string"}}}}
	valid := Node{ID: "plan", Executor: "codex", ReadOnly: true, Instruction: "规划", OutputSchema: summary, ReportSchema: full}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Node){
		func(n *Node) { n.ReadOnly = false },
		func(n *Node) { n.Executor, n.Model = "eino", "test" },
		func(n *Node) { n.OutputSchema = nil },
		func(n *Node) { n.OutputSchema = full },
		func(n *Node) {
			n.OutputSchema = &Shape{Type: "object", Properties: map[string]Shape{"ready": {Type: "string"}}}
		},
	} {
		node := valid
		change(&node)
		if node.Validate() == nil {
			t.Fatal("接受无效报告配置")
		}
	}
}
