package agent

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

func TestTransientModelFailuresUseTrustedErrorEvents(t *testing.T) {
	for _, tt := range []struct{ message, kind string }{
		{"Selected model is at capacity. Please try a different model.", "model_unavailable"},
		{"unexpected status 503 Service Unavailable", "model_unavailable"},
		{"unexpected status 502 Bad Gateway", "model_unavailable"},
		{"unexpected status 504 Gateway Timeout", "model_unavailable"},
		{"unexpected status 429 Too Many Requests", "rate_limit"},
		{"stream disconnected before completion: error sending request for url (https://model.invalid)", "model_network"},
		{"unexpected status 401 Unauthorized", "authentication"},
		{"unexpected status 404 Not Found", "parameter"},
		{"tool exited with status 503", "execution"},
	} {
		t.Run(tt.kind+tt.message, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"type": "turn.failed", "error": map[string]string{"message": tt.message}})
			events := &codexEvents{}
			_, _ = events.Write(append(body, '\n'))
			err := codexFailure(errors.New("模型失败"), events.ErrorStatus)
			if got := workflow.FailureKind(err); got != tt.kind {
				t.Fatalf("分类=%s，期望=%s", got, tt.kind)
			}
		})
	}
	for _, kind := range []string{"agent_message", "command_execution"} {
		body, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]string{"type": kind, "text": "Selected model is at capacity. Please try a different model."}})
		events := &codexEvents{}
		_, _ = events.Write(append(body, '\n'))
		if events.ErrorStatus != 0 {
			t.Fatal("普通模型或工具正文被识别为提供方故障")
		}
	}
}

func TestProviderQuotaIsNotTransientRateLimit(t *testing.T) {
	err := classify(&openai.APIError{HTTPStatusCode: 429, Code: "insufficient_quota", Message: "账户额度不足"})
	if workflow.ModelRetryCategory(workflow.FailureKind(err)) || workflow.CanRetry(err) {
		t.Fatal("账户额度不足会无限重试", err)
	}
	events := &codexEvents{}
	_, _ = events.Write([]byte("{\"type\":\"error\",\"status\":429,\"error\":{\"code\":\"insufficient_quota\",\"message\":\"账户额度不足\"}}\n"))
	err = codexFailure(errors.New("模型调用失败"), events.ErrorStatus)
	if workflow.ModelRetryCategory(workflow.FailureKind(err)) || workflow.CanRetry(err) {
		t.Fatal("原生 Codex 账户额度不足会无限重试", err)
	}
}
