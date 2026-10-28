package scheduler

import (
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func TestSkippedNodesHaveStableIdentifiersAndExecutor(t *testing.T) {
	d := workflow.Definition{Input: `{"enabled":false}`, InputSchema: &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"enabled": {Type: "boolean"}}}, Nodes: []workflow.Node{{ID: "skip", Executor: "eino", Model: "test", Instruction: "执行", When: &workflow.Condition{Field: workflow.FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`true`)}}}}
	run := &RunDetails{RunSummary: RunSummary{ID: uuid.NewString()}, Definition: &d}
	enrichBranches(run, nil)
	if len(run.Nodes) != 1 {
		t.Fatal("缺少跳过节点")
	}
	node := run.Nodes[0]
	if _, err := uuid.Parse(node.ID); err != nil {
		t.Fatal("节点ID不符合API UUID契约")
	}
	if node.TaskID != node.ID || node.Executor != "eino" || node.Attempt != 0 {
		t.Fatalf("派生节点记录不准确 %+v", node)
	}
}
