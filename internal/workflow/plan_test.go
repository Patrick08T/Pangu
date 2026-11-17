package workflow

import "testing"

func TestExecutionPlanRequiresCompleteUnambiguousTaskMapping(t *testing.T) {
	for _, body := range []string{
		`{"tasks":[{"id":"T1"}]}`,
		`{"tasks":[{"id":"T1"}],"execution_plan":{"changes":[],"assignments":[]}}`,
		`{"tasks":[{"id":"T1"}],"execution_plan":{"assignments":[{"task_id":"T2","node_id":"build"}]}}`,
		`{"tasks":[{"id":"T1"}],"execution_plan":{"assignments":[{"task_id":"T1","node_id":"build"},{"task_id":"T1","node_id":"qa"}]}}`,
	} {
		if _, err := ParseExecutionPlan(body); err == nil {
			t.Fatal("接受了未落实的执行计划", body)
		}
	}
	if _, err := ParseExecutionPlan(`{"tasks":[{"id":"T1"}],"execution_plan":{"changes":[],"assignments":[{"task_id":"T1","node_id":"build"}]}}`); err != nil {
		t.Fatal(err)
	}
}
