package api

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

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
	object, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "connection", ProjectID: p.ID, Name: "连接", Spec: catalog.Spec{Connection: &catalog.Connection{BaseURL: "https://example.com/v1", Model: "test", SecretRef: "PANGU_SECRET_TEST"}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, "api-test", object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{}
	request := httptest.NewRequest("POST", "/versions/"+version.ID+"/preflight", strings.NewReader(`{"input":""}`))
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
