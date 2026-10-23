package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStructuredOutputRejectsWrongMissingAndExtraFields(t *testing.T) {
	shape := &Shape{Type: "object", Properties: map[string]Shape{"title": {Type: "string"}, "score": {Type: "number"}}, Required: []string{"title", "score"}}
	node := Node{ID: "pm", Executor: "eino", Model: "test", Instruction: "分析", OutputSchema: shape}
	for _, raw := range []string{`{"title":"分析"}`, `{"title":4,"score":1}`, `{"title":"分析","score":1,"secret":true}`, "```json\n{}\n```"} {
		out := NodeOutput{Text: raw}
		if err := ValidateOutput(node, &out); err == nil {
			t.Errorf("接受错误输出: %s", raw)
		}
	}
	out := NodeOutput{Text: `{"title":"分析","score":2}`}
	if err := ValidateOutput(node, &out); err != nil || out.Data["title"] != "分析" {
		t.Fatalf("结构化输出: %+v %v", out, err)
	}
}

func TestFieldBindingsOnlyReadDirectParentsAndCheckType(t *testing.T) {
	d := Definition{Nodes: []Node{
		{ID: "source", Executor: "eino", Model: "test", Instruction: "来源", OutputSchema: &Shape{Type: "object", Properties: map[string]Shape{"count": {Type: "integer"}}, Required: []string{"count"}}},
		{ID: "target", Executor: "eino", Model: "test", Instruction: "消费", DependsOn: []string{"source"}, Bindings: map[string]FieldBinding{"total": {From: "source", Pointer: "/count", Type: "integer"}}},
	}}
	_, err := Run(d, func(inputs []NodeInput) ([]NodeOutput, error) {
		if inputs[0].Node.ID == "source" {
			return []NodeOutput{{Text: `{"count":3}`}}, nil
		}
		value, _ := json.Marshal(inputs[0].Parameters["total"])
		if string(value) != "3" {
			t.Fatalf("映射值丢失: %+v", inputs[0])
		}
		return []NodeOutput{{Text: "完成"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	d.Nodes[1].DependsOn = nil
	if _, err := d.Layers(); err == nil {
		t.Fatal("无依赖读取被接受")
	}
	d.Nodes[1].DependsOn = []string{"source"}
	d.Nodes[1].Bindings["total"] = FieldBinding{From: "source", Pointer: "/missing", Type: "integer"}
	if _, err := d.BindInput(d.NodeInput(d.Nodes[1], map[string]NodeOutput{"source": {Data: map[string]any{"count": 3.0}}})); err == nil {
		t.Fatal("缺失字段被自动填充")
	}
}

func TestSchemaLimitsAndInputValidation(t *testing.T) {
	shape := &Shape{Type: "object", Properties: map[string]Shape{"goal": {Type: "string"}}, Required: []string{"goal"}}
	d := Definition{InputSchema: shape, Input: `{}`}
	if err := d.ValidateInput(); err == nil {
		t.Fatal("缺少参数被接受")
	}
	d.Input = `{"goal":"调研"}`
	if err := d.ValidateInput(); err != nil {
		t.Fatal(err)
	}
	bad := &Shape{Type: "executable"}
	if _, err := bad.Compile(); err == nil {
		t.Fatal("未知 schema 类型被接受")
	}
	for range 9 {
		bad = &Shape{Type: "array", Items: bad}
	}
	if _, err := bad.Compile(); err == nil {
		t.Fatal("嵌套过深未拒绝")
	}
}

func TestStructuredNumbersCannotChangeDuringJSONTransport(t *testing.T) {
	if fieldType("integer", json.Number("9007199254740992.5")) {
		t.Fatal("小数被 Float64 舍入成整数")
	}
	s := &Shape{Type: "object", Properties: map[string]Shape{"n": {Type: "number"}}, Required: []string{"n"}}
	n := Node{ID: "number", OutputSchema: s}
	for _, raw := range []string{`{"n":9007199254740993}`, `{"n":0.1234567890123456789}`} {
		out := NodeOutput{Text: raw}
		if err := ValidateOutput(n, &out); err == nil {
			t.Fatalf("接受会被 SDK 改写的数值: %s", raw)
		}
	}
	for _, raw := range []string{`{"n":9007199254740991}`, `{"n":0.1}`} {
		out := NodeOutput{Text: raw}
		if err := ValidateOutput(n, &out); err != nil {
			t.Fatalf("拒绝可往返传输数值: %s %v", raw, err)
		}
	}
}

func TestOutputLimitIncludesAssignedRunID(t *testing.T) {
	out := NodeOutput{Text: strings.Repeat("a", 16361)}
	if err := ValidateOutput(Node{ID: "boundary"}, &out); err == nil {
		t.Fatal("未为最终 RunID 预留输出额度")
	}
}
