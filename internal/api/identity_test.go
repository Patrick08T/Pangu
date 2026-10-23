package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/identity"
)

type testAccess struct {
	authorize func(context.Context, string, string, string) error
}

func (a testAccess) Handler(next http.Handler) http.Handler { return next }
func (a testAccess) Authorize(ctx context.Context, project, action, owner string) error {
	if a.authorize != nil {
		return a.authorize(ctx, project, action, owner)
	}
	return nil
}
func testAPI(b Backend) http.Handler { return New(b, testAccess{}) }

func TestInvalidIdentityInputIsBadRequest(t *testing.T) {
	_, memberErr := identity.Grant(context.Background(), identity.Actor{}, testID, "user", "unknown-role")
	_, _, tokenErr := identity.IssueToken(context.Background(), identity.Actor{}, testID, "", []string{"read"}, time.Now().Add(time.Hour))
	_, _, scopeErr := identity.IssueToken(context.Background(), identity.Actor{}, testID, "测试", []string{"read", "read"}, time.Now().Add(time.Hour))
	for _, err := range []error{memberErr, tokenErr, scopeErr} {
		w := httptest.NewRecorder()
		if !backendError(w, err) || w.Code != 400 {
			t.Fatalf("无效身份参数应为 400: %d %v", w.Code, err)
		}
	}
}

func TestProjectActionsCannotBypassAuthorization(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/runs", validRun},
		{"POST", "/workflows/preflight", validRun},
		{"POST", "/crons", `{"name":"daily","expression":"0 0 * * *","workflow":` + validRun + `}`},
		{"GET", "/runs/" + testID, ""},
		{"GET", "/runs/" + testID + "/artifacts", ""},
		{"POST", "/runs/" + testID + "/cancel", ""},
		{"POST", "/runs/" + testID + "/rerun", ""},
	} {
		b := &fakeBackend{}
		h := New(b, testAccess{authorize: func(_ context.Context, _, _, _ string) error { return identity.ErrForbidden }})
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "test-auth")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if len(b.definition.Nodes) != 0 || len(b.cron.Workflow.Nodes) != 0 {
			t.Error("越权请求已提交")
		}
	}
}
