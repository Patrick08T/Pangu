//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func rolloutExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if err := transaction(context.Background(), func(tx pgx.Tx) error { _, err := tx.Exec(context.Background(), sql, args...); return err }); err != nil {
		t.Fatal(err)
	}
}

func TestRolloutRollbackRestoresOnlyOwnedBindingAndReleasesClaims(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	if _, err := f.service.Reconcile(ctx, f.scope, f.id, 1); err != nil {
		t.Fatal(err)
	}
	intent, err := f.service.RequestRollback(ctx, f.scope, f.id, 1, "人工停止观察")
	if err != nil || intent.State != "rolling_back" {
		t.Fatal("rollback intent absent", intent, err)
	}
	_, claims := f.binding(t)
	if claims != 2 {
		t.Fatal("released before restoration", claims)
	}
	intent, err = f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "rolled_back" {
		t.Fatal("rollback did not finish", intent, err)
	}
	b, claims := f.binding(t)
	if b.AdoptedVersion != f.request.BaselineVersion || b.StableVersion != f.request.BaselineVersion || b.IntentID != "" || claims != 0 {
		t.Fatal("rollback left unsafe binding", b, claims)
	}
	if _, err = f.service.RequestRollback(ctx, f.scope, f.id, 1, "重复停止"); err != nil {
		t.Fatal("rollback replay", err)
	}
}

func TestRolloutRollbackNeverOverwritesHumanRevision(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	if _, err := f.service.Reconcile(ctx, f.scope, f.id, 1); err != nil {
		t.Fatal(err)
	}
	rolloutExec(t, `UPDATE pangu.evolution_bindings SET adopted_version=$2,revision=revision+1 WHERE intent_id=$1`, f.id, f.request.BaselineVersion)
	if _, err := f.service.RequestRollback(ctx, f.scope, f.id, 1, "停止"); err != nil {
		t.Fatal(err)
	}
	intent, err := f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "reconciling" {
		t.Fatal("human conflict not persisted", intent, err)
	}
	b, claims := f.binding(t)
	if b.Revision != 3 || b.AdoptedVersion != f.request.BaselineVersion || claims != 2 {
		t.Fatal("human edit overwritten or unresolved claims lost", b, claims)
	}
}

func TestRolloutRevocationAllowsOnlyCompensatingRecovery(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	if _, err := f.service.Reconcile(ctx, f.scope, f.id, 1); err != nil {
		t.Fatal(err)
	}
	f.policy.Revision++
	f.policy.Revoked = true
	f.savePolicy(t)
	if _, err := f.service.Advance(ctx, f.scope, f.id, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("revocation advanced version", err)
	}
	intent, err := f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "rolled_back" {
		t.Fatal("withdrawn policy left owned adoption live", intent, err)
	}
	b, claims := f.binding(t)
	if b.AdoptedVersion != f.request.BaselineVersion || claims != 0 {
		t.Fatal(b, claims)
	}
}
