package agent

import (
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"strings"
	"testing"
)

func TestCodexDefaultZeroUsageStaysUnknown(t *testing.T) {
	events := &codexEvents{}
	_, _ = events.Write([]byte("{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":0,\"output_tokens\":0,\"cached_input_tokens\":0}}\n"))
	if events.Usage != nil {
		t.Fatal("CLI 默认零值被误认为提供方实际用量")
	}
	_, _ = events.Write([]byte("{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":20,\"output_tokens\":4,\"cached_input_tokens\":5}}\n"))
	if events.Usage == nil || events.Usage.Input != 20 {
		t.Fatal("丢失原生 CLI 报告值")
	}
}

func TestCodexClassificationSurvivesDiagnosticRedactionAndTruncation(t *testing.T) {
	for _, message := range []string{strings.Repeat("x", 5000), `token=private-value`} {
		raw, _ := json.Marshal(map[string]any{"type": "error", "status": 400, "error": map[string]string{"code": "model_not_found", "message": message}})
		nested, _ := json.Marshal(map[string]any{"type": "turn.failed", "error": map[string]string{"message": string(raw)}})
		for _, line := range [][]byte{raw, nested} {
			events := &codexEvents{}
			_, _ = events.Write(append(line, '\n'))
			err := codexFailure(errors.New("执行失败"), events.ErrorStatus)
			if workflow.FailureKind(err) != "parameter" || workflow.CanRetry(err) || strings.Contains(events.ErrorMessage, "private-value") || len(events.ErrorMessage) > 4200 {
				t.Fatalf("状态丢失或诊断未脱敏: kind=%s retry=%v size=%d", workflow.FailureKind(err), workflow.CanRetry(err), len(events.ErrorMessage))
			}
		}
	}
}

func TestNativeEventsRequireCompletedCommandExitCode(t *testing.T) {
	events := &codexEvents{}
	input := `{"type":"item.completed","item":{"type":"reasoning","text":"private thinking"}}
{"type":"item.completed","item":{"type":"agent_message","text":"所有测试通过"}}
{"type":"item.completed","item":{"id":"unfinished","type":"command_execution","command":"test","status":"completed"}}
{"type":"item.completed","item":{"id":"proof","type":"command_execution","command":"go test ./...","aggregated_output":"token=private-value","exit_code":1,"status":"failed"}}
`
	for _, part := range []string{input[:21], input[21:67], input[67:]} {
		if _, err := events.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if len(events.Commands) != 1 || events.Commands[0].ExitCode != 1 || strings.Contains(events.Commands[0].Output, "private-value") {
		t.Fatalf("执行证据错误: %+v", events.Commands)
	}
}

func TestCodexFatalEventClassificationIgnoresModelAndToolText(t *testing.T) {
	for _, tt := range []struct {
		event, kind string
		retry       bool
	}{
		{`{"type":"turn.failed","error":{"message":"unexpected status 401 Unauthorized"}}`, "authentication", false},
		{`{"type":"error","message":"unexpected status 403 Forbidden"}`, "authentication", false},
		{`{"type":"turn.failed","error":{"message":"unexpected status 400 Bad Request"}}`, "parameter", false},
		{`{"type":"turn.failed","error":{"message":"unexpected status 429 Too Many Requests"}}`, "rate_limit", true},
		{`{"type":"item.completed","item":{"type":"agent_message","text":"unexpected status 401 Unauthorized"}}`, "execution", true},
		{`{"type":"turn.failed","error":{"message":"unexpected status 500 Internal Server Error"}}`, "execution", true},
		{`{"type":"turn.failed","error":{"message":"{\"type\":\"error\",\"status\":400,\"error\":{\"code\":\"model_not_found\",\"message\":\"unsupported model\"}}"}}`, "parameter", false},
	} {
		events := &codexEvents{}
		_, _ = events.Write([]byte(tt.event + "\n"))
		err := codexFailure(errors.New("执行失败"), events.ErrorStatus)
		if workflow.FailureKind(err) != tt.kind || workflow.CanRetry(err) != tt.retry {
			t.Fatalf("错误分类: %v", err)
		}
	}
}
