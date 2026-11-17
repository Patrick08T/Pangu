//go:build integration

package learning

import (
	"context"
	"testing"
)

func TestMemoryReconcileMarksOnlyConfirmedSourceFailureAsBlocking(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	if err := WithdrawSource(f.ctx, f.scope, version.Sources[0].ID); err != nil {
		t.Fatal(err)
	}
	paused, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, nil)
	if err != nil || !paused {
		t.Fatal("withdrawn source did not pause", paused, err)
	}
	tx := f.transaction(t)
	defer tx.Rollback(f.ctx)
	var block bool
	var validity, hash string
	var bodyCount int
	err = tx.QueryRow(f.ctx, `SELECT coalesce((d.policy->>'block_frozen')::boolean,false),s.validity,v.body_hash,
   (SELECT count(*) FROM pangu.experience_bodies b WHERE b.version_id=v.id)
  FROM pangu.experience_versions v JOIN pangu.experience_deliveries d ON d.version_id=v.id
  JOIN pangu.experience_states s ON s.version_id=v.id
  WHERE v.id=$1 ORDER BY d.revision DESC LIMIT 1`, version.ID).Scan(&block, &validity, &hash, &bodyCount)
	if err != nil || !block || validity != "revoked" || hash != version.BodyHash || bodyCount != 0 {
		t.Fatal("withdrawal lost gate, identity or body erasure", block, validity, hash, bodyCount, err)
	}
}
