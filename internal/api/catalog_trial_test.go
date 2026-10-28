package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func trialObject(t *testing.T) (*catalog.Object, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := workspace.Register(ctx, "api-test", workspace.Project{Name: "工作台测试", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "agent", Name: "测试助手", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Handoff: testHandoff(), Instruction: "原职责", ReadOnly: true}}})
	if err != nil {
		t.Fatal(err)
	}
	return item, p.ID
}

func trialKey() string { return strconv.FormatInt(time.Now().UnixMilli(), 10) + "." + uuid.NewString() }

func trialRequest(server http.Handler, id, project, method, path, key, body string) *httptest.ResponseRecorder {
	if method == "GET" {
		path += "?project_id=" + project
	} else {
		var input map[string]any
		if json.Unmarshal([]byte(body), &input) == nil {
			input["project_id"] = project
			encoded, _ := json.Marshal(input)
			body = string(encoded)
		}
	}
	r := httptest.NewRequest(method, "/catalog/"+id+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	server.ServeHTTP(w, r)
	return w
}

type trialBackend struct{ fakeBackend }

func (b *trialBackend) Submit(ctx context.Context, d workflow.Definition, key string) (*submission.Record, error) {
	return submission.Find(ctx, b.Namespace(), "submit", key, submission.Snapshot{Workflow: d})
}

func TestAgentTrialPinsDraftAndReusesReceiptAfterChange(t *testing.T) {
	item, project := trialObject(t)
	b := &trialBackend{}
	server := New(b, testAccess{})
	key := trialKey()
	body := `{"revision":1,"input":"任务"}`
	w := trialRequest(server, item.ID, project, "POST", "/trials", key, body)
	if w.Code != 202 {
		t.Fatalf("草稿试运行: %d %s", w.Code, w.Body.String())
	}
	var receipt submission.Record
	if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	item.Spec.Agent.Instruction = "新职责"
	if _, err := catalog.Save(context.Background(), "api-test", item.ID, 1, item.Name, item.Spec); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Archive(context.Background(), "api-test", item.ID, true); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		key, body string
		code      int
	}{{key, body, 202}, {key, `{"revision":1,"input":"其他"}`, 409}, {trialKey(), body, 409}} {
		response := trialRequest(server, item.ID, project, "POST", "/trials", test.key, test.body)
		if response.Code != test.code {
			t.Fatalf("重试: %d %s", response.Code, response.Body.String())
		}
		if test.code == 202 && !strings.Contains(response.Body.String(), receipt.ID) {
			t.Fatal("未复用回执")
		}
	}
	stored, err := submission.Get(context.Background(), "api-test", receipt.ID)
	if err != nil || stored.Snapshot.Workflow.Nodes[0].Instruction != "原职责" || stored.Snapshot.Workflow.VersionID != "" {
		t.Fatalf("快照改变: %+v %v", stored, err)
	}
	history := trialRequest(server, item.ID, project, "GET", "/trials", "", "")
	if history.Code != 200 || !strings.Contains(history.Body.String(), receipt.ID) {
		t.Fatalf("历史: %d %s", history.Code, history.Body.String())
	}
	versions, err := catalog.Versions(context.Background(), "api-test", item.ID, 0)
	if err != nil || len(versions) != 0 {
		t.Fatal("试运行污染发布历史", err)
	}
}

func TestAgentTrialPreflightAndAccess(t *testing.T) {
	item, project := trialObject(t)
	b := &trialBackend{}
	for _, path := range []string{"/draft/preflight", "/trials"} {
		server := New(b, testAccess{authorize: func(_ context.Context, project, action, owner string) error { return identity.ErrForbidden }})
		w := trialRequest(server, item.ID, project, "POST", path, trialKey(), `{"revision":1,"input":"任务"}`)
		if w.Code != 403 || b.calls != 0 {
			t.Fatalf("越权: %d %s", w.Code, w.Body.String())
		}
	}
	server := New(b, testAccess{})
	w := trialRequest(server, item.ID, project, "POST", "/draft/preflight", "", `{"revision":1,"input":"任务"}`)
	if w.Code != 200 || b.calls != 1 {
		t.Fatalf("诊断: %d %s", w.Code, w.Body.String())
	}
	list := trialRequest(server, item.ID, project, "GET", "/trials", "", "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"items":[]`) {
		t.Fatal("诊断产生任务", list.Body.String())
	}
	for _, body := range []string{`{"revision":2,"input":"任务"}`, `{"revision":0}`, `{"revision":1,"unknown":true}`} {
		w := trialRequest(server, item.ID, project, "POST", "/trials", trialKey(), body)
		if w.Code < 400 {
			t.Fatal("错误输入被接受", body)
		}
	}
}
