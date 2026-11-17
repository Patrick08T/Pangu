package scheduler

import (
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
	"time"
)

func TestOutputsNeverFallsBackToPreviousReviewRound(t *testing.T) {
	run := &RunDetails{Definition: &workflow.Definition{Nodes: []workflow.Node{{ID: "plan"}}}, Nodes: []NodeRun{
		{NodeID: "plan", RunSummary: RunSummary{ID: "new", Status: "running", CreatedAt: time.Now()}},
		{NodeID: "plan", RunSummary: RunSummary{ID: "old", Status: "succeeded", CreatedAt: time.Now().Add(-time.Hour)}, Output: "旧计划"},
	}}
	if _, ok := Outputs(run)["plan"]; ok {
		t.Fatal("修订运行中仍向下游提供旧计划")
	}
	run.Nodes[0].Status = "succeeded"
	run.Nodes[0].Output = "新计划"
	if Outputs(run)["plan"].Text != "新计划" {
		t.Fatal("旧轮次覆盖新轮次")
	}
}
