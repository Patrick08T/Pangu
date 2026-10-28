package api

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func factoryRequest(h http.Handler, method, path string, in any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(in)
	r := httptest.NewRequest(method, path, strings.NewReader(string(data)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type factoryTestBackend struct{ trialBackend }

func (b *factoryTestBackend) GetRequest(ctx context.Context, id string) (*submission.Record, error) {
	return submission.Get(ctx, b.Namespace(), id)
}

func TestFactoryAPIPersistsAndRetriesOneGenerationWithoutCreatingAgents(t *testing.T) {
	_, project := trialObject(t)
	ctx := context.Background()
	conn, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "connection", Name: "Codex", Spec: catalog.Spec{Connection: &catalog.Connection{Provider: "codex"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, "api-test", conn.ID, conn.Revision)
	if err != nil {
		t.Fatal(err)
	}
	before, err := catalog.List(ctx, "api-test", "agent", 0)
	if err != nil {
		t.Fatal(err)
	}
	h := New(&factoryTestBackend{}, testAccess{})
	w := factoryRequest(h, "POST", "/factory/sessions", map[string]string{"project_id": project})
	var s factory.Session
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &s) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	in := factory.Input{ID: uuid.NewString(), Revision: 1, Message: "创建写作助手", Mode: "build", ConnectionVersion: v.ID}
	w = factoryRequest(h, "POST", "/factory/sessions/"+s.ID+"/messages", in)
	var receipt submission.Record
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	w = factoryRequest(h, "POST", "/factory/sessions/"+s.ID+"/messages", in)
	var again submission.Record
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &again) != nil || again.ID != receipt.ID {
		t.Fatal("消息重试未复用回执", w.Code)
	}
	saved, err := factory.Get(ctx, "api-test", s.ID)
	if err != nil || len(saved.Turns) != 1 || saved.Turns[0].RequestID != receipt.ID {
		t.Fatal("消息或回执未落库", err)
	}
	// 新 handler 代表刷新/重启，恢复的对话不会重新创建消息。
	w = factoryRequest(New(&factoryTestBackend{}, testAccess{}), "GET", "/factory/sessions/"+s.ID, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), in.ID) {
		t.Fatal(w.Code, w.Body.String())
	}
	rows, err := catalog.List(ctx, "api-test", "agent", 0)
	if err != nil || len(rows) != len(before) {
		t.Fatal("生成阶段提前创建资源", err)
	}
}

func TestFactoryProjectIsolationAndViewerReadOnly(t *testing.T) {
	h, ns, project, other, token := authorizationAPI(t)
	for _, p := range []string{project, other} {
		s, err := factory.Create(context.Background(), ns, p)
		if err != nil {
			t.Fatal(err)
		}
		expected := 200
		if p == other {
			expected = 403
		}
		w := callAuthorized(h, "GET", "/factory/sessions/"+s.ID, "", token)
		if w.Code != expected {
			t.Fatal(w.Code, w.Body.String())
		}
		for _, path := range []string{"/factory/sessions", "/factory/sessions/" + s.ID + "/messages", "/factory/sessions/" + s.ID + "/apply"} {
			w = callAuthorized(h, "POST", path, `{"project_id":"`+p+`"}`, token)
			if w.Code != 403 {
				t.Fatal("查看者或跨项目可以修改", path, w.Code, w.Body.String())
			}
		}
	}
}
