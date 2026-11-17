//go:build integration

package experience

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func retentionTestTx(t *testing.T, f storeFixture) pgx.Tx {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	tx, err := conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback(context.Background()) })
	return tx
}

func TestRetentionMissingStateCannotEraseBody(t *testing.T) {
	f := newStoreFixture(t)
	version := f.create(t)
	tx := retentionTestTx(t, f)
	if _, err := tx.Exec(f.ctx, `DELETE FROM pangu.experience_states WHERE version_id=$1`, version.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := EraseVersionsTx(f.ctx, tx, f.scope.Namespace, f.scope.ProjectID, []string{version.ID}, "source_withdrawn"); err == nil {
		t.Fatal("body erased without persisted invalidation")
	}
	var count int
	if err := tx.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_bodies WHERE version_id=$1`, version.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("unknown state allowed destructive side effect")
	}
}

func TestRetentionMixedScopeRejectsWholeBatch(t *testing.T) {
	f, other := newStoreFixture(t), newStoreFixture(t)
	version, foreign := f.create(t), other.create(t)
	tx := retentionTestTx(t, f)
	if _, err := EraseVersionsTx(f.ctx, tx, f.scope.Namespace, f.scope.ProjectID, []string{version.ID, foreign.ID}, "source_withdrawn"); err == nil {
		t.Fatal("mixed-scope batch accepted")
	}
	var count int
	if err := tx.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_bodies WHERE version_id=ANY($1::uuid[])`, []string{version.ID, foreign.ID}).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("invalid batch partly erased payloads", count)
	}
}

func TestRetentionPreservesExistingInvalidationAndRollsBack(t *testing.T) {
	f := newStoreFixture(t)
	version := f.create(t)
	tx := retentionTestTx(t, f)
	if _, err := tx.Exec(f.ctx, `UPDATE pangu.experience_states SET validity='revoked',invalid_reason='original incident' WHERE version_id=$1`, version.ID); err != nil {
		t.Fatal(err)
	}
	out, err := EraseVersionsTx(f.ctx, tx, f.scope.Namespace, f.scope.ProjectID, []string{version.ID}, "retention_expired")
	if err != nil || out.Bodies != 1 {
		t.Fatal(out, err)
	}
	var state, reason, hash string
	if err = tx.QueryRow(f.ctx, `SELECT s.validity,s.invalid_reason,v.body_hash FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id WHERE v.id=$1`, version.ID).Scan(&state, &reason, &hash); err != nil {
		t.Fatal(err)
	}
	if state != "revoked" || reason != "original incident" || hash != version.BodyHash {
		t.Fatal("tombstone identity or reason rewritten", state, reason, hash)
	}
	if err = tx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Read(f.ctx, f.scope, version.ID); err != nil {
		t.Fatal("rollback did not restore body", err)
	}
}
