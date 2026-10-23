package workflow

import "testing"

func TestExecutorValidation(t *testing.T) {
	for _, tt := range []struct {
		executor, model string
		valid           bool
	}{
		{"eino", "model", true}, {"codex", "", true}, {"codex", "model", true},
		{"", "model", false}, {"shell", "model", false}, {"eino", "", false}, {"codex", " ", false},
	} {
		n := Node{ID: "agent", Executor: tt.executor, Model: tt.model, Instruction: "任务"}
		if err := n.Validate(); (err == nil) != tt.valid {
			t.Errorf("executor=%q model=%q err=%v", tt.executor, tt.model, err)
		}
	}
}
