//go:build integration

package experience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestStoreCommitRechecksAuthorizationAndRollsBackAllWrites(t *testing.T) {
	f := newStoreFixture(t)
	version := sampleVersion(uuid.NewString())
	version.ProjectID, version.ExpiresAt = f.scope.ProjectID, time.Now().Add(time.Hour)
	version.Sources[0].ProjectID, version.Sources[0].ID = f.scope.ProjectID, "verified-source"
	var calls atomic.Int32
	f.store.Authorize = func(context.Context, pgx.Tx, Scope) error {
		if calls.Add(1) > 1 {
			return ErrForbidden
		}
		return nil
	}
	if _, err := f.store.Create(f.ctx, f.scope, version); !errors.Is(err, ErrForbidden) {
		t.Fatalf("commit-time authorization ignored: %v", err)
	}
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	var count int
	err = conn.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_versions WHERE id=$1`, version.ID).Scan(&count)
	if err != nil || count != 0 || calls.Load() != 2 {
		t.Fatalf("unauthorized writes persisted: rows=%d calls=%d %v", count, calls.Load(), err)
	}
}

func TestStoreConcurrentPolicyCASHasExactlyOneWinner(t *testing.T) {
	f := newStoreFixture(t)
	policy := enabledPolicy()
	policy.ProjectID, policy.ActorID = f.scope.ProjectID, f.scope.ActorID
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := f.store.SavePolicy(f.ctx, f.scope, policy, 0); results <- err }()
	}
	wins, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			wins++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("CAS race: wins=%d conflicts=%d", wins, conflicts)
	}
}

func TestStoreConcurrentLearningIntentDeduplicatesAcrossConnections(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	input := IntentInput{SourceRunID: uuid.NewString(), EvidenceRevision: 1, PolicyRevision: 1, Purpose: "business", Terminal: true, StopConfirmed: true}
	type result struct {
		intent LearningIntent
		err    error
	}
	results := make(chan result, 6)
	var group sync.WaitGroup
	for range 6 {
		group.Add(1)
		go func() {
			defer group.Done()
			intent, err := f.store.EnsureLearningIntent(f.ctx, f.scope, input)
			results <- result{intent, err}
		}()
	}
	group.Wait()
	close(results)
	ids := map[string]bool{}
	for got := range results {
		if got.err != nil {
			t.Fatal(got.err)
		}
		ids[got.intent.ID] = true
	}
	if len(ids) != 1 {
		t.Fatalf("duplicate persisted intents: %v", ids)
	}
}
