package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func TestRebuildConcurrentCommandsLinkExactlyOneSuccessor(t *testing.T) {
	f := newRebuildFixture(t)
	child := uuid.NewString()
	var wg sync.WaitGroup
	results := make(chan *submission.Record, 3)
	failures := make(chan error, 3)
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) { return child, nil })
			results <- item
			failures <- err
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id string
	for item := range results {
		if id != "" && id != item.ID {
			t.Fatal("concurrent commands escaped intent identity")
		}
		id = item.ID
	}
	if rebuildCount(t, f.actor.Namespace) != 1 {
		t.Fatal("concurrent preparation leaked successor")
	}
}

func TestRebuildRejectsSupersededIntentWithoutOrphan(t *testing.T) {
	f := newRebuildFixture(t)
	if err := approval.ReadyRebuild(f.ctx, f.actor.Namespace, f.run, f.input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	input := f.input
	input.ApprovalGeneration = 2
	if _, err := approval.Request(f.ctx, f.actor.Namespace, f.previous.ID, f.run, input); err != nil {
		t.Fatal(err)
	}
	if _, err := approval.Invalidate(f.ctx, f.actor.Namespace, f.run, input.Node.ID, 2, "another revoked source"); err != nil {
		t.Fatal(err)
	}
	_, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) {
		t.Fatal("superseded intent dispatched")
		return "", nil
	})
	if !errors.Is(err, approval.ErrConflict) || rebuildCount(t, f.actor.Namespace) != 0 {
		t.Fatal("stale intent produced successor", err)
	}
}

func TestRebuildPrepareRollsBackWhenSameRunAlreadyHasNewGate(t *testing.T) {
	f := newRebuildFixture(t)
	if err := approval.ReadyRebuild(f.ctx, f.actor.Namespace, f.run, f.input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	input := f.input
	input.ApprovalGeneration = 2
	if _, err := approval.Request(f.ctx, f.actor.Namespace, f.previous.ID, f.run, input); err != nil {
		t.Fatal(err)
	}
	_, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) {
		t.Fatal("same-run and successor both dispatched")
		return "", nil
	})
	if !errors.Is(err, approval.ErrConflict) || rebuildCount(t, f.actor.Namespace) != 0 {
		t.Fatal("failed link left orphan request", err)
	}
	linked, err := approval.RebuildSuccessor(f.ctx, f.actor.Namespace, f.intent.ID)
	if err != nil || linked != "" {
		t.Fatal("failed link became visible", linked, err)
	}
}

func TestRebuildPendingReceiptNeedsExplicitAuthorizedRetry(t *testing.T) {
	f := newRebuildFixture(t)
	pending, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) {
		return "", errors.New("remote receipt unknown")
	})
	if err == nil || pending == nil || pending.RunID != "" {
		t.Fatal("ambiguous send lost durable intent", pending, err)
	}
	if err = guardRebuildSubmission(context.Background(), f.actor.Namespace, pending, f.stopped); err == nil {
		t.Fatal("background reconciliation acquired user authority")
	}
	rebuildExec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1`, f.actor.ID)
	_, err = rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) {
		t.Fatal("revoked pending intent dispatched")
		return "", nil
	})
	if err == nil || rebuildCount(t, f.actor.Namespace) != 1 {
		t.Fatal("revoked retry escaped guard", err)
	}
}
