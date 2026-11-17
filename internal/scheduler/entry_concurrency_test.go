package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

type entryOutcome struct {
	item *submission.Record
	err  error
}

func TestEntryConcurrentRetriesFreezeExactlyOnce(t *testing.T) {
	f := newEntryFixture(t)
	ctx, cancel := context.WithTimeout(f.ctx, 20*time.Second)
	defer cancel()
	const workers = 8
	var ready sync.WaitGroup
	ready.Add(workers)
	release := make(chan struct{})
	check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
		ready.Done()
		select {
		case <-release:
			return &workflow.Preflight{Ready: true}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	out := make(chan entryOutcome, workers)
	for i := 0; i < workers; i++ {
		go func() {
			item, err := submitEntry(ctx, f.actor.Namespace, f.project, "main", "task", "concurrent", entryHooks{check, entryPending})
			out <- entryOutcome{item, err}
		}()
	}
	allReady := make(chan struct{})
	go func() { ready.Wait(); close(allReady) }()
	select {
	case <-allReady:
	case <-ctx.Done():
		t.Fatal("preflight blocked behind an acceptance transaction")
	}
	close(release)
	var first string
	for i := 0; i < workers; i++ {
		got := <-out
		if got.err != nil {
			t.Fatal(got.err)
		}
		if first == "" {
			first = got.item.ID
		}
		if got.item.ID != first {
			t.Fatal("duplicate acceptance", got.item.ID, first)
		}
	}
	if n := entrySubmissionCount(t, f); n != 1 {
		t.Fatal("duplicate durable requests", n)
	}
	var n int
	if err := entryQuery(t, `SELECT count(*) FROM pangu.experience_watermarks WHERE namespace=$1 AND run_id=$2`, f.actor.Namespace, first).Scan(&n); err != nil || n != 1 {
		t.Fatal("freeze was duplicated", n, err)
	}
}

func TestEntryDispatchFailureLeavesDurableReconcileIntent(t *testing.T) {
	f := newEntryFixture(t)
	failure := errors.New("test transport unavailable")
	dispatch := func(ctx context.Context, item *submission.Record) *submission.Record {
		_, err := submission.Dispatch(ctx, f.actor.Namespace, item.ID, func(ctx context.Context, stored *submission.Record) (string, error) {
			verifyEntryDispatchUnlocked(t, ctx, f, stored.ID)
			return "", failure
		})
		if !errors.Is(err, failure) {
			t.Errorf("unexpected dispatch error: %v", err)
		}
		return item
	}
	item, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "durable", entryHooks{entryReady, dispatch})
	if err != nil || item.Status != "pending" {
		t.Fatal("transport lost durable request", item, err)
	}
	var lastError string
	if err = entryQuery(t, `SELECT last_error FROM pangu.submissions WHERE id=$1`, item.ID).Scan(&lastError); err != nil || lastError == "" {
		t.Fatal("reconcile error not persisted", lastError, err)
	}
	pending, err := submission.Pending(f.ctx, f.actor.Namespace)
	if err != nil || len(pending) != 1 || pending[0] != item.ID {
		t.Fatal("entry missing from durable dispatch reconciliation", pending, err)
	}
}

func verifyEntryDispatchUnlocked(t *testing.T, ctx context.Context, f entryFixture, id string) {
	t.Helper()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	var unlocked, exists bool
	err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,0)),EXISTS(SELECT 1 FROM pangu.submissions WHERE id=$2)`, "identity:"+f.actor.Namespace, id).Scan(&unlocked, &exists)
	if err != nil || !unlocked || !exists {
		t.Fatal("dispatch ran before acceptance committed", unlocked, exists, err)
	}
	err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))`, "experience:"+f.actor.Namespace+":"+f.project).Scan(&unlocked)
	if err != nil || !unlocked {
		t.Fatal("dispatch held experience transaction", unlocked, err)
	}
}
