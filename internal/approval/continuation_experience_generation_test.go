//go:build integration

package approval

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestContinuationMemoryAcrossIndependentPhysicalGenerations(t *testing.T) {
	f := newContinuationMemoryFixture(t)
	item, input, fork := f.start(t)
	input = f.prepare(t, item, input)
	if _, err := Request(f.ctx, f.actor.Namespace, item.ID, item.RunID, input); err != nil {
		t.Fatal(err)
	}
	if _, err := Invalidate(f.ctx, f.actor.Namespace, item.RunID, input.Node.ID, 1, "rebind_before_approval"); err != nil {
		t.Fatal(err)
	}
	if err := ReadyRebuild(f.ctx, f.actor.Namespace, item.RunID, input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	input.ApprovalGeneration = 2
	input = f.prepare(t, item, input)
	parent, err := Request(f.ctx, f.actor.Namespace, item.ID, item.RunID, input)
	if err != nil {
		t.Fatal(err)
	}
	answers := append(acceptedItems(), workflow.ReviewAnswer{ItemID: "platform-execution-graph", Decision: "accept", SelectedOptionIDs: []string{"apply"}})
	if _, err = Decide(f.ctx, f.actor, parent.ID, Decision{Decision: "approved", SubjectHash: parent.SubjectHash, Items: answers}); err != nil {
		t.Fatal(err)
	}
	child, err := Continue(f.ctx, f.actor.Namespace, parent.ID, fork)
	if err != nil {
		t.Fatal(err)
	}
	child = f.dispatch(t, child)
	next := f.prepare(t, child, child.Snapshot.Workflow.NodeInput(child.Snapshot.Workflow.Nodes[1], fork.Reused))
	inherited, err := Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, next)
	if err != nil || inherited.InheritedFrom != parent.ID || inherited.ApprovalGeneration != 1 || parent.ApprovalGeneration != 2 {
		t.Fatal("independent valid generations could not inherit", inherited, err)
	}
}

func TestContinuationMemoryCannotBypassCurrentOwnerOrCancellation(t *testing.T) {
	for _, kind := range []string{"disabled_owner", "cancelled_child", "purged_body"} {
		t.Run(kind, func(t *testing.T) {
			f := newContinuationMemoryFixture(t)
			parent, child, input := f.continuePlan(t)
			switch kind {
			case "disabled_owner":
				f.exec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.actor.ID)
			case "cancelled_child":
				f.exec(t, `INSERT INTO pangu.run_cancellations(run_id,namespace) VALUES($1,$2)`, child.RunID, f.actor.Namespace)
			case "purged_body":
				f.exec(t, `DELETE FROM pangu.experience_bodies WHERE version_id=$1`, f.version.ID)
			}
			gate, err := Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, input)
			if kind == "cancelled_child" {
				// Request may persist history; cancellation is enforced at consumption.
				if err != nil || gate.Status != "approved" {
					t.Fatal("cancel fixture did not reach consumption boundary", gate, err)
				}
				if err = Consume(f.ctx, child.RunID, input, gate.ID); err == nil {
					t.Fatal("cancelled child consumed inherited approval")
				}
				f.cancellationRetained(t, child.RunID)
				return
			}
			if err == nil {
				t.Fatal("unusable authority inherited approval")
			}
			f.noInheritedGate(t, parent, child.RunID)
		})
	}
}

func (f continuationMemoryFixture) cancellationRetained(t *testing.T, run string) {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	var count int
	if err = conn.QueryRow(f.ctx, `SELECT count(*) FROM pangu.run_cancellations WHERE run_id=$1`, run).Scan(&count); err != nil || count != 1 {
		t.Fatal("cancel was automatically cleared", count, err)
	}
}
