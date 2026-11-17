package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func prepareGenerationSuccessor(t *testing.T, actor identity.Actor, run, key string) *submission.Record {
	t.Helper()
	ctx := context.Background()
	old, err := submission.ForRun(ctx, actor.Namespace, run)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := old.Snapshot
	snapshot.SourceRunID, snapshot.ApprovalOrigin = run, ""
	next, err := submission.Prepare(ctx, actor.Namespace, "rebuild", key, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestApprovalRebuildLinksFreshRunWithoutInheritedApproval(t *testing.T) {
	actor, request, run, input, _ := generationFixture(t, "approved")
	ctx := context.Background()
	intent, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked")
	if err != nil {
		t.Fatal(err)
	}
	next := prepareGenerationSuccessor(t, actor, run, "rebuild-one")
	if err = LinkRebuild(ctx, actor.Namespace, intent.ID, next.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("unready intent linked", err)
	}
	if err = ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = LinkRebuild(ctx, actor.Namespace, intent.ID, next.ID); err != nil {
			t.Fatal("idempotent successor link failed", err)
		}
	}
	other := prepareGenerationSuccessor(t, actor, run, "rebuild-two")
	if err = LinkRebuild(ctx, actor.Namespace, intent.ID, other.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("successor remapped", err)
	}
	child := uuid.NewString()
	if _, err = submission.Dispatch(ctx, actor.Namespace, next.ID, func(context.Context, *submission.Record) (string, error) { return child, nil }); err != nil {
		t.Fatal(err)
	}
	recovered, err := RebuildSuccessor(ctx, actor.Namespace, intent.ID)
	if err != nil || recovered != next.ID {
		t.Fatalf("successor recovery failed: %s %v", recovered, err)
	}
	if _, err = Request(ctx, actor.Namespace, request, run, generationInput(t, input, 2)); !errors.Is(err, ErrConflict) {
		t.Fatal("mapped intent also rebuilt the old run", err)
	}
	changed := input
	changed.Node.Instruction = "different authority scope"
	if _, err = Request(ctx, actor.Namespace, next.ID, child, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("successor escaped frozen authority", err)
	}
	gate, err := Request(ctx, actor.Namespace, next.ID, child, input)
	if err != nil || gate.Status != "waiting" || gate.InheritedFrom != "" || gate.ApprovalGeneration != 1 {
		t.Fatalf("fresh successor inherited an approval: %+v %v", gate, err)
	}
	if err = LinkRebuild(ctx, actor.Namespace, intent.ID, next.ID); err != nil {
		t.Fatal("existing link could not recover after dispatch", err)
	}
}

func TestApprovalRebuildRejectsLateSuccessorBinding(t *testing.T) {
	actor, _, run, input, _ := generationFixture(t, "waiting")
	ctx := context.Background()
	intent, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked")
	if err != nil {
		t.Fatal(err)
	}
	if err = ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	next := prepareGenerationSuccessor(t, actor, run, "late-link")
	_, err = submission.Dispatch(ctx, actor.Namespace, next.ID, func(context.Context, *submission.Record) (string, error) {
		return "", errors.New("receipt lost after external send")
	})
	if err == nil {
		t.Fatal("dispatch fixture should fail")
	}
	if err = LinkRebuild(ctx, actor.Namespace, intent.ID, next.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("possibly dispatched request linked late", err)
	}
}
