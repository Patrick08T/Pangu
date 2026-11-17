package catalog

import (
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func TestPlanRejectsHistoricalEditsAndDanglingTasks(t *testing.T) {
	flow := &Flow{Nodes: []Binding{{ID: "plan", AgentVersion: "agent"}, {ID: "build", AgentVersion: "agent", DependsOn: []string{"plan"}}}}
	for _, p := range []workflow.ExecutionPlan{
		{Changes: []workflow.PlanChange{{NodeID: "plan", Remove: true}}},
		{Changes: []workflow.PlanChange{{NodeID: "build", Remove: true}}},
		{Assignments: []workflow.TaskAssignment{{TaskID: "T1", NodeID: "missing"}}},
		{Assignments: []workflow.TaskAssignment{{TaskID: "T1", NodeID: "plan"}}},
	} {
		if _, _, err := patchPlan(flow, p, map[string]bool{"plan": true}, "build"); err == nil {
			t.Fatal("计划改变了历史、审批或虚构分工", p)
		}
	}
	task := "实施获批任务"
	p := workflow.ExecutionPlan{Changes: []workflow.PlanChange{{NodeID: "build", Task: &task}}, Assignments: []workflow.TaskAssignment{{TaskID: "T1", NodeID: "build"}}}
	updated, diff, err := patchPlan(flow, p, map[string]bool{"plan": true}, "build")
	if err != nil || len(diff) != 1 || updated.Nodes[1].Task != task || flow.Nodes[1].Task != "" {
		t.Fatalf("执行图修订污染原版本: %+v %v", diff, err)
	}
}
