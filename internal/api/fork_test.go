package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"net/http/httptest"
	"strings"
	"testing"
)

type sourceRemovedBackend struct{ fakeBackend }

func (*sourceRemovedBackend) GetRun(context.Context, string) (*scheduler.RunDetails, error) {
	return nil, scheduler.ErrNotFound
}

func TestForkReceiptRouteDoesNotRequireSourceHistory(t *testing.T) {
	b := &sourceRemovedBackend{}
	r := httptest.NewRequest("POST", "/runs/"+testID+"/fork", strings.NewReader(`{"checkpoint_node":"writer","workflow":`+validRun+`}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "fork-receipt")
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, r)
	if w.Code != 202 || b.calls != 1 {
		t.Fatalf("来源已清理时不能访问返工回执 %d %s", w.Code, w.Body.String())
	}
}
