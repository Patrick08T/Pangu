//go:build integration

package approval

import (
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (f continuationMemoryFixture) continuePlan(t *testing.T) (*Gate, *submission.Record, workflow.NodeInput) {
	t.Helper()
	return f.continuePrepared(t, true, true)
}

func (f continuationMemoryFixture) continuePrepared(t *testing.T, prepareParent, prepareChild bool, before ...func()) (*Gate, *submission.Record, workflow.NodeInput) {
	t.Helper()
	parent, input, fork := f.start(t)
	if prepareParent {
		input = f.prepare(t, parent, input)
	}
	gate, err := Request(f.ctx, f.actor.Namespace, parent.ID, parent.RunID, input)
	if err != nil {
		t.Fatal(err)
	}
	items := append(acceptedItems(), workflow.ReviewAnswer{ItemID: "platform-execution-graph", Decision: "accept", SelectedOptionIDs: []string{"apply"}})
	_, err = Decide(f.ctx, f.actor, gate.ID, Decision{Decision: "approved", SubjectHash: gate.SubjectHash, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range before {
		change()
	}
	next, err := Continue(f.ctx, f.actor.Namespace, gate.ID, fork)
	if err != nil {
		t.Fatal(err)
	}
	next = f.dispatch(t, next)
	childInput := next.Snapshot.Workflow.NodeInput(next.Snapshot.Workflow.Nodes[1], fork.Reused)
	childInput.RequestID, childInput.ApprovalRound, childInput.ApprovalGeneration = next.ID, 1, 1
	if prepareChild {
		childInput = f.prepare(t, next, childInput)
	}
	return gate, next, childInput
}

// The real consumer must inherit approved memory, not compare it to a synthetic empty payload.
func TestContinuationInheritsAuthoritativelyFrozenMemory(t *testing.T) {
	f := newContinuationMemoryFixture(t)
	parent, child, input := f.continuePlan(t)
	gate, err := Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, input)
	if err != nil {
		t.Fatalf("same frozen memory could not inherit approved execution: %v", err)
	}
	if gate.Status != "approved" || gate.InheritedFrom != parent.ID || gate.ApprovalGeneration != 1 {
		t.Fatalf("wrong inherited authorization: %+v", gate)
	}
	var subject struct {
		Input workflow.NodeInput `json:"node_input"`
	}
	if err = json.Unmarshal(gate.Subject, &subject); err != nil {
		t.Fatal(err)
	}
	if len(subject.Input.Experiences) != 1 || subject.Input.Experiences[0].Reference.VersionID != f.version.ID {
		t.Fatal("inherited subject lost actual approved memory")
	}
	old, err := Get(f.ctx, f.actor.Namespace, parent.ID)
	if err != nil || old.Status != "consumed" {
		t.Fatal("parent approval history changed", old, err)
	}
	if err = Consume(f.ctx, child.RunID, input, gate.ID); err != nil {
		t.Fatal("valid inherited gate was not consumable", err)
	}
}
