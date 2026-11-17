package scheduler

import (
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestPlanRevisionRetainsFrozenLogicalAttempt(t *testing.T) {
	plan := workflow.Node{ID: "plan", Instruction: "draft", Executor: "codex", ReadOnly: true}
	apply := workflow.Node{ID: "apply", Instruction: "apply", Executor: "codex", DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}
	d := workflow.Definition{ProjectID: "123e4567-e89b-12d3-a456-426614174000", Input: "task", Nodes: []workflow.Node{plan, apply}}
	original := d.NodeInput(apply, nil)
	original.RequestID, original.ApprovalGeneration = "accepted-request", 4
	original.Iteration = &workflow.Iteration{ID: "cycle", Round: 2}
	body, err := json.Marshal(map[string]any{"node_input": original})
	if err != nil {
		t.Fatal(err)
	}
	gate := approval.Gate{ID: "decision", NodeID: "apply", Round: 2, ApprovalGeneration: 4, Subject: body, Review: &workflow.Review{}}
	calls := 0
	err = revisePlan(d, apply, gate, map[string]workflow.NodeOutput{}, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		calls++
		in := inputs[0]
		if in.RequestID != "accepted-request" || in.ApprovalGeneration != 4 || in.Iteration == nil || in.Iteration.ID != "cycle" || in.Iteration.Round != 2 {
			t.Fatal("revision lost frozen logical attempt", in)
		}
		if in.ReviewFeedback == nil || in.ReviewFeedback.ApprovalID != "decision" || in.ReviewFeedback.Round != 3 {
			t.Fatal("revision lost persisted decision", in)
		}
		return []workflow.NodeOutput{{Text: "revised plan"}}, nil
	})
	if err != nil || calls != 1 {
		t.Fatal("revision failed", calls, err)
	}
}
