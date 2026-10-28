package workflow

import "testing"

func TestPolicyValidatesRetries(t *testing.T) {
	n := Node{ID: "one", Executor: "eino", Model: "test", Instruction: "任务"}
	if n.EffectivePolicy().Retries != 2 {
		t.Fatal("默认策略发生变化")
	}
	for _, p := range []Policy{{Retries: 3}, {Retries: -1}} {
		n.Policy = &p
		if err := n.Validate(); err == nil {
			t.Fatalf("越界策略被接受: %+v", p)
		}
	}
	n.Policy = &Policy{Retries: 0}
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
}
