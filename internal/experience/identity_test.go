package experience

import "testing"

func TestSelectionIdentityPinsNodeRoundGenerationAndBoundInputs(t *testing.T) {
	input := SelectionInput{ProjectID: "project", RunID: "request", NodeID: "node", Round: 1,
		Generation: 1, ApprovalRevision: 1, BoundInputHash: "bound", Text: "first"}
	key := selectionIdentity(input)
	input.Text, input.ErrorText = "retry", "changed diagnostic"
	if selectionIdentity(input) != key {
		t.Fatal("retry reranked because text changed")
	}
	for _, change := range []func(*SelectionInput){
		func(in *SelectionInput) { in.NodeID = "other" },
		func(in *SelectionInput) { in.Round++ },
		func(in *SelectionInput) { in.Generation++ },
		func(in *SelectionInput) { in.ApprovalRevision++ },
		func(in *SelectionInput) { in.BoundInputHash = "other" },
	} {
		changed := input
		change(&changed)
		if selectionIdentity(changed) == key {
			t.Fatal("execution identity boundary omitted")
		}
	}
}
