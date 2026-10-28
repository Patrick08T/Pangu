package scheduler_test

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestHatchetNodeEventsArePagedAndOwnedByRoot(t *testing.T) {
	_, backend, h := integrationSetup(t)
	project := registerTestProject(t, backend)
	var accepted struct {
		RunID string `json:"run_id"`
	}
	request(t, h, "POST", "/runs", workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "one", Instruction: "输出"}}}, 202, &accepted)
	run := waitRun(t, h, accepted.RunID)
	var page scheduler.EventPage
	request(t, h, "GET", "/runs/"+run.ID+"/events?node_run_id="+run.Nodes[0].ID+"&limit=1", nil, 200, &page)
	if len(page.Items) != 1 || page.Items[0].TaskID != run.Nodes[0].TaskID || page.NextOffset == nil {
		t.Fatalf("节点事件分页无效: %+v", page)
	}
	if _, err := backend.Events(context.Background(), run.ID, uuid.NewString(), 0, 20); err == nil {
		t.Fatal("允许查询无关运行的节点")
	}
}
