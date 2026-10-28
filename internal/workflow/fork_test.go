package workflow

import "testing"

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
