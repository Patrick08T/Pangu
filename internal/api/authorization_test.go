package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type authorizedBackend struct {
	fakeBackend
	namespace string
}

func (b *authorizedBackend) Namespace() string { return b.namespace }

func authorizationAPI(t *testing.T) (http.Handler, string, string, string, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	for k, v := range map[string]string{"PANGU_FEISHU_APP_ID": "test", "PANGU_FEISHU_APP_SECRET": "test", "PANGU_FEISHU_TENANT_KEY": "tenant", "PANGU_PUBLIC_URL": "https://pangu.example"} {
		t.Setenv(k, v)
	}
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	ns, project, other, user := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{project, other} {
		_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,$2,'test','test','main','worker')`, id, ns)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1,$2,'test','tenant','ou_viewer')`, user, ns)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.memberships(user_id,project_id,role) VALUES($1,$2,'viewer')`, user, project)
	if err != nil {
		t.Fatal(err)
	}
	token := uuid.NewString() + "session"
	hash := sha256.Sum256([]byte(token))
	_, err = conn.Exec(ctx, `INSERT INTO pangu.sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '1 hour')`, hex.EncodeToString(hash[:]), user)
	if err != nil {
		t.Fatal(err)
	}
	return New(&authorizedBackend{namespace: ns}, identity.Service{Namespace: ns}), ns, project, other, token
}

func callAuthorized(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.AddCookie(&http.Cookie{Name: "pangu_session", Value: token})
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://pangu.example")
	r.Header.Set("Idempotency-Key", "authorization-test")
	if strings.HasSuffix(path, "/trials") {
		r.Header.Set("Idempotency-Key", trialKey())
	}
	hash := sha256.Sum256([]byte("pangu-csrf:" + token))
	r.Header.Set("X-CSRF-Token", hex.EncodeToString(hash[:]))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAuthenticatedSharedCatalogPreservesWorkspaceAndWriteBoundaries(t *testing.T) {
	h, ns, project, other, token := authorizationAPI(t)
	var owned, foreign *catalog.Object
	for _, id := range []string{project, other} {
		item, err := catalog.Create(context.Background(), ns, catalog.NewObject{Kind: "agent", Name: "共享 Agent", Spec: catalog.Spec{Agent: &workflow.Node{ID: "one", Executor: "codex", Handoff: testHandoff(), Instruction: "任务"}}})
		if err != nil {
			t.Fatal(err)
		}
		if id == project {
			owned = item
		} else {
			foreign = item
		}
	}
	w := callAuthorized(h, "GET", "/catalog", "", token)
	var result page[catalog.Object]
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || len(result.Items) != 2 {
		t.Fatalf("列表泄漏或缺失: %d %s", w.Code, w.Body.String())
	}
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/catalog/" + foreign.ID, ""},
		{"GET", "/catalog/" + foreign.ID + "/export", ""},
		{"POST", "/catalog/" + owned.ID + "/publish", `{"revision":1}`},
		{"POST", "/runs", strings.ReplaceAll(validRun, testID, project)},
	} {
		w = callAuthorized(h, tc.method, tc.path, tc.body, token)
		expected := 403
		if tc.method == "GET" {
			expected = 200
		}
		if w.Code != expected {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	w = callAuthorized(h, "POST", "/auth/logout", "", token)
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	w = callAuthorized(h, "GET", "/catalog", "", token)
	if w.Code != 401 {
		t.Fatal("退出未立即生效")
	}
}
