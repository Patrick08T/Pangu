package agent

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestModelRequestHasNoBusinessDeadline(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "local-test-only")
	config, err := modelConfig(workflow.Node{Executor: "eino", Model: "local"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.Timeout != 0 || config.HTTPClient.Timeout != 0 {
		t.Fatalf("模型请求仍受固定执行时限约束：%v / %v", config.Timeout, config.HTTPClient.Timeout)
	}
}
