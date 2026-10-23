package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestRerunRejectsChunkedOverrides(t *testing.T) {
	backend := &fakeBackend{}
	request := httptest.NewRequest("POST", "/runs/"+testID+"/rerun", strings.NewReader(`{"input":"覆盖"}`))
	request.ContentLength = -1
	request.Header.Set("Idempotency-Key", "rerun-test")
	response := httptest.NewRecorder()
	testAPI(backend).ServeHTTP(response, request)
	if response.Code != 400 || backend.calls != 0 {
		t.Fatalf("分块请求绕过原快照限制: %d calls=%d", response.Code, backend.calls)
	}
}

func (b *fakeBackend) ListRuns(context.Context, scheduler.RunQuery) (*scheduler.RunList, error) {
	b.calls++
	return &scheduler.RunList{Items: []scheduler.RunSummary{}}, b.err
}

func (b *fakeBackend) CancelRun(context.Context, string) (*scheduler.Cancellation, error) {
	b.calls++
	return &scheduler.Cancellation{RunID: testID, Requested: true}, b.err
}

func (b *fakeBackend) Preflight(_ context.Context, definition workflow.Definition) (*workflow.Preflight, error) {
	b.calls++
	b.definition = definition
	return &workflow.Preflight{Ready: true, Checks: []workflow.Check{}}, b.err
}

func TestCancellationAcknowledgesRequestOnly(t *testing.T) {
	backend := &fakeBackend{}
	response := httptest.NewRecorder()
	testAPI(backend).ServeHTTP(response, httptest.NewRequest("POST", "/runs/"+testID+"/cancel", nil))
	if response.Code != 202 || backend.calls != 2 {
		t.Fatalf("取消路由: %d %s", response.Code, response.Body.String())
	}
}

func TestRunListValidationAndEmptyPage(t *testing.T) {
	for _, item := range []struct {
		query string
		want  int
	}{
		{"", 200}, {"?status=running&limit=100", 200}, {"?limit=101", 400},
		{"?status=COMPLETED", 400}, {"?offset=-1", 400}, {"?until=bad", 400},
	} {
		backend := &fakeBackend{}
		response := httptest.NewRecorder()
		testAPI(backend).ServeHTTP(response, httptest.NewRequest("GET", "/runs"+item.query, nil))
		if response.Code != item.want || item.want == 400 && backend.calls != 0 {
			t.Errorf("%s: %d %s", item.query, response.Code, response.Body.String())
		}
	}
}
