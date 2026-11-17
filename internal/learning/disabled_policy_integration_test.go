//go:build integration

package learning

import (
	"context"
	"testing"
	"time"
)

func TestLearningDisabledDoesNotRevokeAuthorizedExistingMemory(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	source := version.Sources[0]
	policy, err := f.store.Policy(f.ctx, f.scope)
	if err != nil {
		t.Fatal(err)
	}
	previous := policy.Revision
	policy.Enabled = false
	if _, err = f.store.SavePolicy(f.ctx, f.scope, policy, previous); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadSource(f.ctx, f.scope, source); err != nil {
		t.Fatal("disabled learning revoked authorized existing source", err)
	}
	if err = GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err == nil {
		t.Fatal("disabled learning permitted fresh consent")
	}
	paused, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, nil)
	if err != nil || !paused {
		t.Fatal("disabled learning did not ordinary pause", paused, err)
	}
	current, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil || current.Validity != "active" || current.BodyHash != version.BodyHash {
		t.Fatal("ordinary pause revoked validity", current, err)
	}
	assertNoFrozenBlock(t, f, version.ID)
}

func assertNoFrozenBlock(t *testing.T, f learningFixture, id string) {
	t.Helper()
	tx := f.transaction(t)
	defer tx.Rollback(f.ctx)
	var blocked bool
	err := tx.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pangu.experience_deliveries WHERE version_id=$1 AND policy->'block_frozen'='true'::jsonb)`, id).Scan(&blocked)
	if err != nil || blocked {
		t.Fatal("ordinary pause wrote immutable frozen block", blocked, err)
	}
}
