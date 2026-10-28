package api

import (
	"github.com/Patrick08T/Pangu/internal/workflow"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandoffRequiredForRawPreflight(t *testing.T) {
	b := &fakeBackend{}
	r := httptest.NewRequest("POST", "/workflows/preflight", strings.NewReader(`{"project_id":"eb1d38c7-d454-4bc7-a337-827b76a5e6c8","input":"检查","nodes":[{"id":"agent","executor":"codex","instruction":"检查"}]}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "missing-handoff")
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, r)
	if w.Code != 400 || b.calls != 0 || !strings.Contains(w.Body.String(), "交接契约") {
		t.Fatalf("原始 API 绕过必填契约: code=%d calls=%d body=%s", w.Code, b.calls, w.Body.String())
	}
}

func testHandoff() *workflow.HandoffContract {
	return &workflow.HandoffContract{Objective: "完成指定任务", Inputs: "本次输入和上游产物", Deliverables: "任务结果", Acceptance: "结果符合本次要求且说明验证证据", FailureHandling: "明确缺口，不得伪造完成"}
}
