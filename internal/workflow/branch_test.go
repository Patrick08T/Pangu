package workflow

import (
	"encoding/json"
	"testing"
)

func TestBranchSkipsUnselectedPathAndNeverCallsIt(t *testing.T) {
	d := Definition{Input: `{"enabled":true}`, InputSchema: &Shape{Type: "object", Properties: map[string]Shape{"enabled": {Type: "boolean"}}, Required: []string{"enabled"}}, Nodes: []Node{
		{ID: "yes", Executor: "eino", Model: "test", Instruction: "执行", When: &Condition{Field: FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`true`)}},
		{ID: "no", Executor: "eino", Model: "test", Instruction: "执行", When: &Condition{Field: FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`false`)}},
		{ID: "child", Executor: "eino", Model: "test", Instruction: "执行", DependsOn: []string{"no"}},
	}}
	calls := 0
	out, err := Run(d, func(inputs []NodeInput) ([]NodeOutput, error) {
		calls++
		if len(inputs) != 1 || inputs[0].Node.ID != "yes" {
			t.Fatalf("执行未选分支 %+v", inputs)
		}
		return []NodeOutput{{Text: "完成"}}, nil
	})
	if err != nil || calls != 1 || !out["no"].Skipped || !out["child"].Skipped {
		t.Fatalf("跳过结果 %+v %v", out, err)
	}
	d.Nodes[0].When.Equals = json.RawMessage(`"true"`)
	if _, err = d.Layers(); err == nil {
		t.Fatal("条件类型不匹配")
	}
}

func TestPreviewPointsToMissingFieldWithoutExecuting(t *testing.T) {
	d := Definition{Input: `{}`, InputSchema: &Shape{Type: "object"}, Nodes: []Node{{ID: "one", Executor: "eino", Model: "test", Instruction: "执行", Bindings: map[string]FieldBinding{"topic": {From: "$input", Pointer: "/missing", Type: "string"}}}}}
	previews, err := PreviewInputs(d, nil)
	if err != nil || len(previews) != 1 || previews[0].Error == "" || previews[0].NodeID != "one" {
		t.Fatalf("未定位绑定错误 %+v %v", previews, err)
	}
}
