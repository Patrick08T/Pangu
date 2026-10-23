package agent

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"strings"
	"testing"
)

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
	} {
		events := &codexEvents{}
		_, _ = events.Write([]byte(tt.event + "\n"))
		err := codexFailure(errors.New("执行失败"), events.ErrorMessage)
		if workflow.FailureKind(err) != tt.kind || workflow.CanRetry(err) != tt.retry {
			t.Fatalf("错误分类: %v", err)
		}
	}
}
