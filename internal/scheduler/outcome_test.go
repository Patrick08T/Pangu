package scheduler

import (
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	"testing"
)

func TestReportReferenceSurvivesRunProjectionForAcceptance(t *testing.T) {
	var row rest.V1TaskSummary
	raw := `{"status":"COMPLETED","additionalMetadata":{"pangu_node_id":"acceptance"},"output":{"text":"{\"iteration_complete\":false,\"status\":\"partial\"}","report_ids":["report"],"report_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatal(err)
	}
	shape := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"iteration_complete": {Type: "boolean"}, "status": {Type: "string"}}}
	d := workflow.Definition{Nodes: []workflow.Node{{ID: "acceptance", ReportSchema: shape, OutputSchema: shape}}}
	run := &RunDetails{Definition: &d, Nodes: []NodeRun{nodeRun(row)}}
	out := workflow.BusinessOutcome(d, Outputs(run), "succeeded")
	if out.State != "needs_attention" {
		t.Fatalf("报告引用投影丢失使部分验收消失：%+v", out)
	}
}
