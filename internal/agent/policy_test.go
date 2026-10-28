package agent

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

func TestModelErrorsPreserveRetryClassification(t *testing.T) {
	for _, item := range []struct {
		status   int
		category string
		retry    bool
	}{{400, "parameter", false}, {401, "authentication", false}, {403, "authentication", false}, {429, "rate_limit", true}, {503, "execution", true}} {
		err := classify(&openai.APIError{HTTPStatusCode: item.status, Message: "测试错误"})
		if workflow.FailureKind(err) != item.category || workflow.CanRetry(err) != item.retry {
			t.Fatalf("%d 分类错误: %v", item.status, err)
		}
	}
	if err := classify(context.DeadlineExceeded); workflow.FailureKind(err) != "timeout" || !workflow.CanRetry(err) {
		t.Fatal("超时类型丢失")
	}
}

func TestCodexPolicyExitCodesNeverRetry(t *testing.T) {
	for code, kind := range map[int]string{88: "trace", 87: "tool_authority"} {
		err := codexFailure(processExitError(code), 0)
		if workflow.FailureKind(err) != kind || workflow.CanRetry(err) {
			t.Fatalf("策略退出码 %d 未终止重试: %v", code, err)
		}
	}
}
