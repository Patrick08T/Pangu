package approval

import (
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestApprovalRebuildKeepsAuthorityScope(t *testing.T) {
	old := workflow.NodeInput{ProjectID: "project", Input: "task", Node: workflow.Node{ID: "build", Role: "implementation", Executor: "codex"}}
	for _, change := range []func(*workflow.NodeInput){
		func(in *workflow.NodeInput) { in.Node.Role = "release" },
		func(in *workflow.NodeInput) { in.Node.ReadOnly = true },
		func(in *workflow.NodeInput) { in.Node.Executor = "command" },
		func(in *workflow.NodeInput) { in.ProjectID = "other" },
		func(in *workflow.NodeInput) { in.Input = "other" },
		func(in *workflow.NodeInput) { in.ReviewPurpose = "evidence" },
	} {
		next := old
		change(&next)
		if sameRebuildScope(old, next) {
			t.Fatal("rebuild changed frozen authority scope")
		}
	}
	next := old
	next.ApprovalGeneration = 2
	next.Parents = map[string]workflow.NodeOutput{"plan": {Text: "new clean plan"}}
	next.Parameters = map[string]any{"plan": "new clean plan"}
	if !sameRebuildScope(old, next) {
		t.Fatal("clean upstream reconstruction was rejected")
	}
}

func TestApprovalIgnoresPhysicalRequestIdentity(t *testing.T) {
	old := workflow.NodeInput{Input: "logical task", ApprovalGeneration: 1}
	next := old
	next.RequestID = "new-physical-request"
	a, err := inputHash(old)
	if err != nil {
		t.Fatal(err)
	}
	b, err := inputHash(next)
	if err != nil || a != b {
		t.Fatal("physical request changed logical approval hash", err)
	}
	if !sameInheritedInput(old, next) {
		t.Fatal("physical request prevented inherited approval")
	}
}

// Dropping generation from the approval input hash must break this contract.
func TestApprovalGenerationHashSeparatesNewGeneration(t *testing.T) {
	var hashes []string
	for _, generation := range []int{0, 1, 2} {
		raw, err := json.Marshal(map[string]any{"approval_generation": generation})
		if err != nil {
			t.Fatal(err)
		}
		var input workflow.NodeInput
		if err = json.Unmarshal(raw, &input); err != nil {
			t.Fatal(err)
		}
		hash, err := inputHash(input)
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, hash)
	}
	if hashes[0] != hashes[1] {
		t.Fatal("initial zero and explicit generation one must preserve legacy hashes")
	}
	if hashes[1] == hashes[2] {
		t.Fatal("new generation reused the old approval input hash")
	}
}
