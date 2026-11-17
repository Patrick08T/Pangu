package workflow

import "testing"

func TestContinuationStopsOldGraphAndPreservesCompletedResults(t *testing.T) {
	d := Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Nodes: []Node{{ID: "plan", Executor: "codex", Instruction: "规划", ReadOnly: true}, {ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}}}}
	calls := 0
	out, err := RunControlled(d, nil, func(input *NodeInput, outputs map[string]NodeOutput) error {
		if input.Node.ID == "build" {
			return ErrContinued
		}
		return nil
	}, func(inputs []NodeInput) ([]NodeOutput, error) {
		calls++
		return []NodeOutput{{Text: "已完成计划"}}, nil
	})
	if err != nil || calls != 1 || out["plan"].Text != "已完成计划" {
		t.Fatalf("旧执行图仍实施或丢失成果: %+v %v", out, err)
	}
}
