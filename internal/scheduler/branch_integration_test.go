package scheduler_test

import (
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func TestHatchetBranchNeverDispatchesSkippedNodes(t *testing.T) {
	model, backend, h := integrationSetup(t)
	project := registerTestProject(t, backend)
	d := workflow.Definition{ProjectID: project.ID, Input: `{"enabled":true}`, Budget: &workflow.Budget{ModelCalls: 1}, InputSchema: &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"enabled": {Type: "boolean"}}, Required: []string{"enabled"}}, Nodes: []workflow.Node{
		{ID: "yes", Executor: "eino", Model: "yes", Handoff: testHandoff(), Instruction: "执行", When: &workflow.Condition{Field: workflow.FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`true`)}},
		{ID: "no", Executor: "eino", Model: "no", Handoff: testHandoff(), Instruction: "不应执行", When: &workflow.Condition{Field: workflow.FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`false`)}},
		{ID: "child", Executor: "eino", Model: "child", Handoff: testHandoff(), Instruction: "不应执行", DependsOn: []string{"no"}},
	}}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	request(t, h, "POST", "/runs", d, 202, &accepted)
	run := waitRun(t, h, accepted.RunID)
	if !run.StopConfirmed || len(run.Nodes) != 3 || model.count("yes") != 1 || model.count("no") != 0 || model.count("child") != 0 {
		t.Fatalf("错误派发: %+v", run)
	}
	for _, node := range run.Nodes {
		if node.NodeID != "yes" && node.Status != "skipped" {
			t.Fatalf("跳过状态错误 %+v", node)
		}
	}
	var inputs []workflow.InputPreview
	request(t, h, "GET", "/runs/"+run.ID+"/inputs", nil, 200, &inputs)
	if len(inputs) != 3 {
		t.Fatal("缺少节点输入预览")
	}
}
