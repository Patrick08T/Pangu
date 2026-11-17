package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
)

func TestApprovalInvalidationAndConsumptionSerialize(t *testing.T) {
	actor, _, run, input, gate := generationFixture(t, "approved")
	ctx := context.Background()
	start := make(chan struct{})
	consumed := make(chan error, 1)
	go func() {
		<-start
		consumed <- Consume(ctx, run, input, gate.ID)
	}()
	close(start)
	intent, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked")
	consumeErr := <-consumed
	if err != nil || (consumeErr != nil && !errors.Is(consumeErr, ErrConflict)) {
		t.Fatal(err, consumeErr)
	}
	old, err := Get(ctx, actor.Namespace, gate.ID)
	if err != nil || old.InvalidatedAt == nil {
		t.Fatal("missing invalidation marker", err)
	}
	if intent.RequiresStop != (consumeErr == nil) || (old.ConsumedAt != nil) != (consumeErr == nil) {
		t.Fatal("lost consumption/stop requirement during invalidation", consumeErr)
	}
	if err = Consume(ctx, run, input, gate.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("late consumption replay succeeded", err)
	}
}

func TestApprovalInvalidationDoesNotEraseHumanCorrection(t *testing.T) {
	actor, _, run, input, gate := generationFixture(t, "changes_requested")
	ctx := context.Background()
	before, err := Get(ctx, actor.Namespace, gate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked"); err != nil {
		t.Fatal(err)
	}
	after, err := Get(ctx, actor.Namespace, gate.ID)
	if err != nil || before.Reason != after.Reason || !sameAnswers(before.Items, after.Items) || before.Status != after.Status {
		t.Fatalf("human decision overwritten: %+v %+v %v", before, after, err)
	}
	state, err := RunState(ctx, run)
	if err != nil || state != "" {
		t.Fatal("invalidated correction still drives human revision", state, err)
	}
}

func TestApprovalGenerationReadinessRejectsUnknownAndStaleGenerations(t *testing.T) {
	actor, request, run, input, _ := generationFixture(t, "waiting")
	ctx := context.Background()
	if err := ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); !errors.Is(err, ErrConflict) {
		t.Fatal("unplanned generation marked ready", err)
	}
	if _, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 2, "future"); !errors.Is(err, ErrConflict) {
		t.Fatal("invalidation skipped generation", err)
	}
	first, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked")
	if err != nil {
		t.Fatal(err)
	}
	if err = CheckGeneration(ctx, actor.Namespace, run, input.Node.ID, 2); !errors.Is(err, ErrConflict) {
		t.Fatal("unready generation considered dispatchable", err)
	}
	if err = ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	if _, err = Request(ctx, actor.Namespace, request, run, generationInput(t, input, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err = Invalidate(ctx, actor.Namespace, run, input.Node.ID, 2, "revoked again"); err != nil {
		t.Fatal(err)
	}
	if err = ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); !errors.Is(err, ErrConflict) {
		t.Fatal("stale readiness acknowledged", err)
	}
	retry, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "different retry reason")
	if err != nil || retry.ID != first.ID || retry.Reason != first.Reason {
		t.Fatal("late invalidation duplicated or overwrote intent", err)
	}
}

func TestApprovalInvalidationRetentionPurgesReason(t *testing.T) {
	actor, _, run, input, gate := generationFixture(t, "waiting")
	ctx := context.Background()
	if _, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked"); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.approvals SET subject=NULL WHERE id=$1", gate.ID); err != nil {
		t.Fatal(err)
	}
	gate, err = Get(ctx, actor.Namespace, gate.ID)
	if err != nil || gate.InvalidationReason != "" {
		t.Fatal("purged gate retained invalidation payload", err)
	}
	intents, err := RebuildIntents(ctx, actor.Namespace, run)
	if err != nil || len(intents) != 1 || intents[0].Reason != "" {
		t.Fatal("purged intent retained reason payload", err)
	}
}
