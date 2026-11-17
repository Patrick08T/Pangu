//go:build integration

package learning

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestRetentionWithdrawalPhysicallyErasesDerivedBodies(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	other := newLearningFixture(t)
	unrelated, _, otherChecker := other.claim(t)
	if err := WithdrawSource(f.ctx, f.scope, version.Sources[0].ID); err != nil {
		t.Fatal(err)
	}
	tx := f.transaction(t)
	var bodies, checkers int
	var revoked, tombstoned bool
	var validity, hash string
	if err := tx.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM pangu.experience_bodies WHERE version_id=$1),
  (SELECT count(*) FROM pangu.learning_checker_bodies WHERE checker_id=$2),
  (SELECT revoked_at IS NOT NULL FROM pangu.learning_source_consents WHERE namespace=$3 AND project_id=$4 AND source_id=$5),
  s.validity,s.tombstoned,v.body_hash FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id WHERE v.id=$1`,
		version.ID, checker.ID, f.scope.Namespace, f.scope.ProjectID, version.Sources[0].ID).Scan(&bodies, &checkers, &revoked, &validity, &tombstoned, &hash); err != nil {
		t.Fatal(err)
	}
	if bodies != 0 || checkers != 0 || !revoked || validity != "revoked" || !tombstoned || hash != version.BodyHash {
		t.Fatalf("withdrawal retained private payload or lost tombstone: bodies=%d checkers=%d revoked=%t state=%s/%t hash=%s", bodies, checkers, revoked, validity, tombstoned, hash)
	}
	if err := tx.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM pangu.experience_bodies WHERE version_id=$1),
  (SELECT count(*) FROM pangu.learning_checker_bodies WHERE checker_id=$2)`, unrelated.ID, otherChecker.ID).Scan(&bodies, &checkers); err != nil {
		t.Fatal(err)
	}
	if bodies != 1 || checkers != 1 {
		t.Fatal("withdrawal erased another project", bodies, checkers)
	}
	if _, err := f.store.Read(f.ctx, f.scope, version.ID); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatal("erased identity remained readable", err)
	}
}

func TestRetentionWithdrawalDoesNotEraseUnauthorizedScope(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	f.exec(t, `UPDATE pangu.memberships SET role='viewer' WHERE user_id=$1 AND project_id=$2`, f.actor.ID, f.scope.ProjectID)
	if err := WithdrawSource(f.ctx, f.scope, version.Sources[0].ID); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("viewer erased source", err)
	}
	tx := f.transaction(t)
	var count int
	if err := tx.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_bodies WHERE version_id=$1`, version.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("denied withdrawal still erased body")
	}
	if err := tx.QueryRow(f.ctx, `SELECT count(*) FROM pangu.learning_checker_bodies WHERE checker_id=$1`, checker.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("denied withdrawal erased checker")
	}
}
