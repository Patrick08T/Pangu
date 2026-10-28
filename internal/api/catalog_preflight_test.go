package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func TestVersionInputValidationReturnsBadRequestBeforeWorker(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := workspace.Register(ctx, "api-test", workspace.Project{Name: "输入校验", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "connection", Name: "Codex", Spec: catalog.Spec{Connection: &catalog.Connection{Provider: "codex"}}})
	if err != nil {
		t.Fatal(err)
	}
	cv, err := catalog.Publish(ctx, "api-test", connection.ID, connection.Revision)
	if err != nil {
		t.Fatal(err)
	}
	node := &workflow.Node{ID: "pm", Executor: "eino", Handoff: testHandoff(), Instruction: "规划", Role: "product_manager", Mode: "planning", ConnectionVersion: cv.ID}
	a, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "agent", Name: "产品经理", Spec: catalog.Spec{Agent: node}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, "api-test", a.ID, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	brief := workflow.ResearchBrief{Question: "规划", Audience: "开发者", Market: "Agent", TimeScope: "最近半年", Context: "无", Constraints: "易用性"}
	for _, route := range []string{"preflight", "run"} {
		for _, invalid := range []bool{true, false} {
			brief.PublicQueries = nil
			if invalid {
				brief.PublicQueries = []string{" "}
			}
			input, _ := json.Marshal(brief)
			body, _ := json.Marshal(map[string]string{"project_id": p.ID, "input": string(input)})
			r := httptest.NewRequest("POST", "/versions/"+v.ID+"/"+route, bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Idempotency-Key", "input-validation")
			w, backend := httptest.NewRecorder(), &fakeBackend{}
			testAPI(backend).ServeHTTP(w, r)
			if invalid {
				if w.Code != 400 || backend.calls != 0 || !strings.Contains(w.Body.String(), "允许公开发送的搜索词第 1 项") {
					t.Errorf("%s 未返回具体输入错误: %d %s calls=%d", route, w.Code, w.Body.String(), backend.calls)
				}
			} else if w.Code < 200 || w.Code >= 300 || backend.calls != 1 {
				t.Errorf("%s 合法输入未到 Worker: %d %s calls=%d", route, w.Code, w.Body.String(), backend.calls)
			}
		}
	}
}

func TestConnectionVersionPreflightUsesWorkerWithoutRunningModel(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := workspace.Register(ctx, "api-test", workspace.Project{Name: "连接诊断", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_CONNECTION_GRANTS", `[{"project_id":"`+p.ID+`","base_url":"https://example.com/v1","secret_ref":"PANGU_SECRET_TEST","models":["test"]}]`)
	object, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "connection", Name: "连接", Spec: catalog.Spec{Connection: &catalog.Connection{BaseURL: "https://example.com/v1", Model: "test", SecretRef: "PANGU_SECRET_TEST"}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, "api-test", object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{}
	request := httptest.NewRequest("POST", "/versions/"+version.ID+"/preflight", strings.NewReader(`{"project_id":"`+p.ID+`","input":""}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	testAPI(backend).ServeHTTP(response, request)
	if response.Code != 200 || backend.calls != 1 {
		t.Fatalf("诊断未到 Worker: %d %s", response.Code, response.Body.String())
	}
	if backend.definition.ProjectID != p.ID || len(backend.definition.Nodes) != 1 || backend.definition.Nodes[0].ConnectionVersion != version.ID {
		t.Fatalf("诊断未固定连接: %+v", backend.definition)
	}
}
