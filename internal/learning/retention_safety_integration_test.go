//go:build integration

package learning

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestRetentionErrorsNeverAcknowledgeErasure(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	expired, expiredChecker := f.retentionExpiredCopy(t, version, checker)
	tx := f.transaction(t)
	if _, err := tx.Exec(f.ctx, `SELECT 1/0`); err == nil {
		t.Fatal("test transaction did not abort")
	}
	if _, err := PurgeExpiredTx(f.ctx, tx, f.scope.Namespace, f.scope.ProjectID, 10); err == nil {
		t.Fatal("SQL error disguised as successful retention")
	}
	tx.Rollback(f.ctx)
	tx = f.transaction(t)
	ctx, cancel := context.WithCancel(f.ctx)
	cancel()
	if _, err := PurgeExpiredTx(ctx, tx, f.scope.Namespace, f.scope.ProjectID, 10); err == nil {
		t.Fatal("cancelled erasure acknowledged")
	}
	tx.Rollback(f.ctx)
	if a, b := f.retentionCounts(t, expired, expiredChecker); a != 1 || b != 1 {
		t.Fatal("failed maintenance deleted bytes", a, b)
	}
}

func TestRetentionWithdrawalCannotRegrantOrReuseErasedID(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	if err := WithdrawSource(f.ctx, f.scope, version.Sources[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := GrantSource(f.ctx, f.scope, version.Sources[0], time.Now().Add(time.Hour)); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("withdrawn exact source was reauthorized", err)
	}
	if _, err := f.store.Create(f.ctx, f.scope, version); err == nil {
		t.Fatal("erased ID reused for body")
	}
	if err := WithdrawSource(f.ctx, f.scope, version.Sources[0].ID); err != nil {
		t.Fatal("idempotent withdrawal failed", err)
	}
	if a, b := f.retentionCounts(t, version.ID, checker.ID); a != 0 || b != 0 {
		t.Fatal("withdrawn payload restored", a, b)
	}
}

func TestRetentionDisabledActorDoesNotExtendExpiry(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	expired, expiredChecker := f.retentionExpiredCopy(t, version, checker)
	f.exec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.actor.ID)
	tx := f.transaction(t)
	if _, err := PurgeExpiredTx(context.Background(), tx, f.scope.Namespace, f.scope.ProjectID, 10); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if a, b := f.retentionCounts(t, expired, expiredChecker); a != 0 || b != 0 {
		t.Fatal("disabled manager extended retained bytes", a, b)
	}
}
