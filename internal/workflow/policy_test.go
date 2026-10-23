package workflow

import "testing"

func TestPolicyCannotExceedConfirmedLimits(t *testing.T) {
	n := Node{ID: "one", Executor: "eino", Model: "test", Instruction: "任务"}
	if n.EffectivePolicy().Retries != 2 || n.EffectivePolicy().TimeoutSeconds != 180 {
		t.Fatal("默认策略发生变化")
	}
	for _, p := range []Policy{{Retries: 3, TimeoutSeconds: 60, ToolCalls: 1}, {Retries: 1, TimeoutSeconds: 181, ToolCalls: 1}, {Retries: 1, TimeoutSeconds: 60, ToolCalls: 11}, {Retries: -1, TimeoutSeconds: 60}} {
		n.Policy = &p
		if err := n.Validate(); err == nil {
			t.Fatalf("越界策略被接受: %+v", p)
		}
	}
	n.Policy = &Policy{Retries: 0, TimeoutSeconds: 1, ToolCalls: 0}
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
}
