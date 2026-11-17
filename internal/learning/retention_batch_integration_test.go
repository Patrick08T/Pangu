//go:build integration

package learning

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func TestRetentionMaintenanceIsBoundedAndIdempotent(t *testing.T) {
	f := newLearningFixture(t)
	version, _, checker := f.claim(t)
	first, firstChecker := f.retentionExpiredCopy(t, version, checker)
	second, secondChecker := f.retentionExpiredCopy(t, version, checker)
	for iteration := 0; iteration < 3; iteration++ {
		tx := f.transaction(t)
		out, err := PurgeExpiredTx(context.Background(), tx, f.scope.Namespace, f.scope.ProjectID, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := int64(1)
		if iteration == 2 {
			want = 0
		}
		if out.Bodies != want || out.CheckerBodies != want {
			t.Fatal("maintenance limit or repeat incorrect", iteration, out)
		}
		if err = tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	if a, b := f.retentionCounts(t, first, firstChecker); a != 0 || b != 0 {
		t.Fatal("first expired payload retained", a, b)
	}
	if a, b := f.retentionCounts(t, second, secondChecker); a != 0 || b != 0 {
		t.Fatal("second expired payload retained", a, b)
	}
	if a, b := f.retentionCounts(t, version.ID, checker.ID); a != 1 || b != 1 {
		t.Fatal("maintenance erased live payload", a, b)
	}
}

func TestRetentionWithdrawalSameSourceIDCannotCrossProject(t *testing.T) {
	f, other := newLearningFixture(t), newLearningFixture(t)
	version, _, _ := f.claim(t)
	foreign, _, checker := other.claim(t)
	id := uuid.NewString()
	source := version.Sources[0]
	source.ProjectID = other.scope.ProjectID
	sources, _ := json.Marshal([]experience.Source{source})
	other.exec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by)
  SELECT $1,namespace,project_id,$1,1,body_hash,$2,expires_at,created_by FROM pangu.experience_versions WHERE id=$3`, id, sources, foreign.ID)
	other.exec(t, `INSERT INTO pangu.experience_states(version_id,validity) VALUES($1,'candidate')`, id)
	other.exec(t, `INSERT INTO pangu.experience_bodies(version_id,content) SELECT $1,content FROM pangu.experience_bodies WHERE version_id=$2`, id, foreign.ID)
	if err := WithdrawSource(f.ctx, f.scope, source.ID); err != nil {
		t.Fatal(err)
	}
	if a, b := other.retentionCounts(t, id, checker.ID); a != 1 || b != 1 {
		t.Fatal("same source ID/hash crossed project boundary", a, b)
	}
}
