package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"net/http/httptest"
	"testing"
)

func TestSharedCodexModelsDoNotRequireProject(t *testing.T) {
	b := &fakeBackend{}
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, httptest.NewRequest("GET", "/models/codex", nil))
	if w.Code != 200 || b.calls != 1 {
		t.Fatalf("模型目录仍依赖项目: %d %s", w.Code, w.Body.String())
	}
}

func (b *fakeBackend) CodexModels(context.Context) ([]workflow.ModelChoice, error) {
	b.calls++
	return []workflow.ModelChoice{{ID: "test-model", Name: "测试模型"}}, b.err
}

func TestCodexModelChoicesAreWorkspaceScoped(t *testing.T) {
	h, _, project, other, token := authorizationAPI(t)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"project_id=" + project, 400}, {"project_id=" + other, 400}, {"", 200}, {"project_id=invalid", 400}, {"project_id=" + project + "&path=other", 400},
	} {
		w := callAuthorized(h, "GET", "/models/codex?"+tc.query, "", token)
		if w.Code != tc.status {
			t.Errorf("%s: %d %s", tc.query, w.Code, w.Body.String())
		}
	}
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.memberships SET role='viewer' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w := callAuthorized(h, "GET", "/models/codex", "", token); w.Code != 200 {
		t.Fatalf("查看者无法读取共享模型目录：%d", w.Code)
	}
}
