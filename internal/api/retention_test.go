package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/identity"
	"net/http/httptest"
	"testing"
)

func TestRetentionRequiresAdministratorForPreviewAndExecution(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"GET", "/retention"}, {"POST", "/retention"}, {"GET", "/retention/history"}} {
		backend := &fakeBackend{}
		h := New(backend, testAccess{authorize: func(_ context.Context, _, action, _ string) error {
			if action != "admin" {
				t.Errorf("清理授权错误: %s", action)
			}
			return identity.ErrForbidden
		}})
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
		if response.Code != 403 || backend.calls != 0 {
			t.Fatalf("非管理员调用清理: %d calls=%d", response.Code, backend.calls)
		}
	}
}
