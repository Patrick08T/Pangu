package scheduler

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestReplayReceiptUsesCollectorWithMultipleDeliverySinks(t *testing.T) {
	d := workflow.Definition{Nodes: []workflow.Node{
		{ID: "collector", Role: "product_manager", Mode: "research"},
		{ID: "feishu", DependsOn: []string{"collector"}},
		{ID: "wechat", DependsOn: []string{"collector"}},
	}}
	plan := evolution.ExecutionPlan{Definition: d, Environment: evolution.Environment{Backend: "eino_research_replay_v1"}}
	outputs := map[string]workflow.NodeOutput{"collector": {Text: "verified claims"}}
	id, text, err := evolutionResultOutput(plan, outputs, true)
	if err != nil || id != "collector" || text != "verified claims" {
		t.Fatalf("id=%s text=%s err=%v", id, text, err)
	}
	outputs["feishu"] = workflow.NodeOutput{}
	if _, _, err := evolutionResultOutput(plan, outputs, true); err == nil {
		t.Fatal("收据不能接纳执行了交付节点的回放")
	}
}
