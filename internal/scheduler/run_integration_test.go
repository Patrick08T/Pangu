package scheduler_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetRunListAndNamespaceIsolation(t *testing.T) {
	_, backend, h := integrationSetup(t)
	project := registerTestProject(t, backend)
	var accepted struct {
		RunID string `json:"run_id"`
	}
	definition := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "one", Instruction: "输出"}}}
	request(t, h, "POST", "/runs", definition, 202, &accepted)
	details := waitRun(t, h, accepted.RunID)
	q := scheduler.RunQuery{Since: time.Now().Add(-time.Hour), Until: time.Now(), Limit: 1}
	page, err := backend.ListRuns(context.Background(), q)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != accepted.RunID || page.NextOffset != nil {
		t.Fatalf("list: %+v %v", page, err)
	}
	request(t, h, "GET", "/runs/"+details.Nodes[0].ID, nil, 404, nil)
	_, other, _ := integrationSetup(t)
	response := httptest.NewRecorder()
	api.New(other, integrationAccess{}).ServeHTTP(response, httptest.NewRequest("GET", "/runs/"+accepted.RunID, nil))
	if response.Code != 404 {
		t.Fatalf("跨命名空间泄漏: %d %s", response.Code, response.Body.String())
	}
	page, err = other.ListRuns(context.Background(), q)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("foreign list: %+v %v", page, err)
	}
}
