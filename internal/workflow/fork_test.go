package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestForkInvalidatesChangedNodeAndAllItsDescendants(t *testing.T) {
	previous := Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Input: "原输入", Nodes: []Node{{ID: "plan", Executor: "codex", Instruction: "规划"}, {ID: "build", Executor: "codex", Instruction: "实现", DependsOn: []string{"plan"}}, {ID: "review", Executor: "codex", Instruction: "审查", DependsOn: []string{"build"}}}}
	changed := previous
	changed.Nodes = append([]Node{}, previous.Nodes...)
	changed.Nodes[1].Instruction = "修复接口"
	reused, err := Prefix(changed, previous, "build")
	if err != nil || len(reused) != 1 || reused[0] != "plan" {
		t.Fatalf("影响范围 %+v %v", reused, err)
	}
	changed.Input = "新输入"
	reused, err = Prefix(changed, previous, "build")
	if err != nil || len(reused) != 0 {
		t.Fatalf("全局输入变化仍复用 %+v %v", reused, err)
	}
	changed = previous
	calls := 0
	out, err := RunFrom(changed, map[string]NodeOutput{"plan": {Text: "已有方案"}, "build": {Text: "已有实现"}}, func(inputs []NodeInput) ([]NodeOutput, error) {
		calls++
		if len(inputs) != 1 || inputs[0].Node.ID != "review" || inputs[0].Parents["build"].Text != "已有实现" {
			t.Fatal("重放或丢失检查点")
		}
		return []NodeOutput{{Text: "审查完成"}}, nil
	})
	if err != nil || calls != 1 || len(out) != 3 {
		t.Fatalf("恢复 %+v %v", out, err)
	}
}

func TestRecoveredStructuredPrefixRetainsConditionsAndBindings(t *testing.T) {
	d := Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Nodes: []Node{
		{ID: "plan", Executor: "codex", Instruction: "规划"},
		{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"},
			When:     &Condition{Field: FieldBinding{From: "plan", Pointer: "/ready", Type: "boolean"}, Equals: json.RawMessage(`true`)},
			Bindings: map[string]FieldBinding{"goal": {From: "plan", Pointer: "/summary", Type: "string"}}},
	}}
	for _, ready := range []bool{false, true} {
		summary := strings.Repeat("完整计划", 700)
		text, _ := json.Marshal(map[string]any{"ready": ready, "summary": summary})
		encoded, _ := json.Marshal(map[string]NodeOutput{"plan": {Text: string(text), RunID: "00000000-0000-0000-0000-000000000001"}})
		var restored map[string]NodeOutput
		if err := json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		previews, err := PreviewInputs(d, restored)
		if err != nil || len(previews) != 2 || previews[1].Error != "" {
			t.Fatalf("检查点输入预览失败: %+v %v", previews, err)
		}
		calls := 0
		out, err := RunFrom(d, restored, func(inputs []NodeInput) ([]NodeOutput, error) {
			calls++
			if len(inputs) != 1 || inputs[0].Node.ID != "build" || inputs[0].Parameters["goal"] != summary {
				t.Fatal("恢复后重复规划或丢失条件/字段")
			}
			return []NodeOutput{{Text: "完成"}}, nil
		})
		if err != nil || (calls == 1) != ready || out["build"].Skipped == ready {
			t.Fatalf("恢复路由错误，ready=%t, calls=%d: %v", ready, calls, err)
		}
	}
}

func TestForkPreservesResearchBetweenFileCheckpoints(t *testing.T) {
	d := Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Nodes: []Node{
		{ID: "context", Executor: "codex", Instruction: "现状", ReadOnly: true},
		{ID: "research", Executor: "eino", Model: "test", Instruction: "研究", DependsOn: []string{"context"}},
		{ID: "plan", Executor: "codex", Instruction: "规划", ReadOnly: true, DependsOn: []string{"research"}},
		{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}},
	}}
	ids, err := Prefix(d, d, "plan")
	if err != nil || strings.Join(ids, ",") != "context,research,plan" {
		t.Fatalf("研究节点使已完成计划无法续用: %v %v", ids, err)
	}
	changed := d
	changed.Nodes = append([]Node{}, d.Nodes...)
	changed.Nodes[1].Instruction = "新研究范围"
	ids, err = Prefix(changed, d, "plan")
	if err != nil || strings.Join(ids, ",") != "context" {
		t.Fatalf("研究变更未使下游失效: %v %v", ids, err)
	}
}
