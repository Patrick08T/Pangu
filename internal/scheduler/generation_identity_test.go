package scheduler

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestNodeRunIdentityFencesApprovalGenerations(t *testing.T) {
	input := workflow.NodeInput{Node: workflow.Node{ID: "implement", Executor: "codex"}}
	first, _ := nodeRunIdentity(input)
	input.ApprovalGeneration = 1
	explicit, _ := nodeRunIdentity(input)
	if explicit != first {
		t.Fatal("initial generation changed existing run identity")
	}
	input.ApprovalGeneration = 2
	next, metadata := nodeRunIdentity(input)
	if next == first || metadata["pangu_approval_generation"] != "2" {
		t.Fatalf("new generation can reuse an old durable child: %s %+v", next, metadata)
	}
}
