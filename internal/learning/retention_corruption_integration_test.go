//go:build integration

package learning

import (
	"context"
	"testing"
)

func TestRetentionMissingStateCannotAcknowledgeExpiry(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	expired, expiredChecker := f.retentionExpiredCopy(t, version, checker)
	f.exec(t, `DELETE FROM pangu.experience_states WHERE version_id=$1`, expired)
	tx := f.transaction(t)
	if _, err := PurgeExpiredTx(context.Background(), tx, f.scope.Namespace, f.scope.ProjectID, 10); err == nil {
		t.Fatal("missing invalidation proof disguised as successful expiry")
	}
	tx.Rollback(f.ctx)
	if a, b := f.retentionCounts(t, expired, expiredChecker); a != 1 || b != 1 {
		t.Fatal("failed expiry partly erased bytes", a, b)
	}
}

func TestRetentionMissingStateCannotAcknowledgeWithdrawal(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	f.exec(t, `DELETE FROM pangu.experience_states WHERE version_id=$1`, version.ID)
	if err := WithdrawSource(f.ctx, f.scope, version.Sources[0].ID); err == nil {
		t.Fatal("missing invalidation proof disguised as successful withdrawal")
	}
	if a, b := f.retentionCounts(t, version.ID, checker.ID); a != 1 || b != 1 {
		t.Fatal("failed withdrawal partly erased bytes", a, b)
	}
	var revoked bool
	if err := f.transaction(t).QueryRow(f.ctx, `SELECT revoked_at IS NOT NULL FROM pangu.learning_source_consents WHERE namespace=$1 AND project_id=$2 AND source_id=$3`, f.scope.Namespace, f.scope.ProjectID, version.Sources[0].ID).Scan(&revoked); err != nil || revoked {
		t.Fatal("failed withdrawal committed partial consent mutation", revoked, err)
	}
}
