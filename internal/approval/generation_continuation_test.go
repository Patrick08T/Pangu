package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func inheritedGenerationFixture(t *testing.T) (identity.Actor, string, workflow.NodeInput, *Gate, *Gate) {
	t.Helper()
	actor, run, request, d, input, fork := planFixture(t)
	ctx := context.Background()
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	items := append(acceptedItems(), workflow.ReviewAnswer{ItemID: "platform-execution-graph", Decision: "accept", SelectedOptionIDs: []string{"apply"}})
	if _, err = Decide(ctx, actor, gate.ID, Decision{Decision: "approved", SubjectHash: gate.SubjectHash, Items: items}); err != nil {
		t.Fatal(err)
	}
	next, err := Continue(ctx, actor.Namespace, gate.ID, fork)
	if err != nil {
		t.Fatal(err)
	}
	child := uuid.NewString()
	if _, err = submission.Dispatch(ctx, actor.Namespace, next.ID, func(context.Context, *submission.Record) (string, error) { return child, nil }); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready)VALUES($1,$2,'head','head','test','child',true)`, child, d.ProjectID); err != nil {
		t.Fatal(err)
	}
	childInput := next.Snapshot.Workflow.NodeInput(next.Snapshot.Workflow.Nodes[1], fork.Reused)
	childGate, err := Request(ctx, actor.Namespace, next.ID, child, childInput)
	if err != nil || childGate.Status != "approved" || childGate.InheritedFrom != gate.ID {
		t.Fatalf("inheritance fixture failed: %+v %v", childGate, err)
	}
	return actor, child, childInput, gate, childGate
}

func TestApprovalInvalidationRejectsContinuationReceiptReplays(t *testing.T) {
	actor, _, _, old, _ := inheritedGenerationFixture(t)
	ctx := context.Background()
	if _, err := Invalidate(ctx, actor.Namespace, old.RunID, old.NodeID, 1, "dependency revoked"); err != nil {
		t.Fatal(err)
	}
	if _, err := Continue(ctx, actor.Namespace, old.ID, nil); !errors.Is(err, ErrConflict) {
		t.Fatal("late plan continuation receipt accepted", err)
	}
	if _, err := ContinueRepair(ctx, actor.Namespace, old.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("late repair continuation receipt accepted", err)
	}
}

func TestApprovalInvalidationRejectsPreviouslyInheritedAuthorization(t *testing.T) {
	actor, child, childInput, old, inherited := inheritedGenerationFixture(t)
	ctx := context.Background()
	if _, err := Invalidate(ctx, actor.Namespace, old.RunID, old.NodeID, 1, "dependency revoked"); err != nil {
		t.Fatal(err)
	}
	if err := Consume(ctx, child, childInput, inherited.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("inherited authorization survived revoked source", err)
	}
}
