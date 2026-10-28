package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecutionPolicyContainsOnlyRetryDecision(t *testing.T) {
	raw, err := json.Marshal((Node{Executor: "codex"}).EffectivePolicy())
	if err != nil || strings.Contains(string(raw), "timeout_seconds") || strings.Contains(string(raw), "tool_calls") {
		t.Fatalf("执行策略仍暴露已取消的时限或工具计数：%s %v", raw, err)
	}
}
