//go:build integration

package learning

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func TestRetentionConsentExpiryMatchesExactSourceRevision(t *testing.T) {
	for _, cause := range []string{"expired", "withdrawn", "expired_uppercase"} {
		t.Run(cause, func(t *testing.T) {
			f := newLearningFixture(t)
			current, _, checker := f.claim(t)
			old := f.retentionOldSourceCopy(t, current, cause)
			tx := f.transaction(t)
			out, err := PurgeExpiredTx(context.Background(), tx, f.scope.Namespace, f.scope.ProjectID, 10)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			if out.Bodies != 1 || out.CheckerBodies != 1 {
				t.Fatal("expired consent retained derived payload", out)
			}
			if a, b := f.retentionCounts(t, old, checker.ID); a != 0 || b != 0 {
				t.Fatal("source-derived rows remain", a, b)
			}
			if a, _ := f.retentionCounts(t, current.ID, checker.ID); a != 1 {
				t.Fatal("old consent erased new source revision")
			}
			var validity string
			if err := f.transaction(t).QueryRow(f.ctx, `SELECT validity FROM pangu.experience_states WHERE version_id=$1`, old).Scan(&validity); err != nil {
				t.Fatal(err)
			}
			want := "expired"
			if cause == "withdrawn" {
				want = "revoked"
			}
			if validity != want {
				t.Fatal("retention changed invalidation semantics", validity, want)
			}
		})
	}
}

func (f learningFixture) retentionOldSourceCopy(t *testing.T, current experience.Version, cause string) string {
	t.Helper()
	old := current.Sources[0]
	old.Hash = digest([]byte("old exact source revision"))
	expired := strings.HasPrefix(cause, "expired")
	f.exec(t, `INSERT INTO pangu.learning_source_consents(id,namespace,project_id,source_id,source_kind,run_id,node_id,source_hash,actor_id,expires_at,revoked_at)
  SELECT $1,namespace,project_id,source_id,source_kind,run_id,node_id,$2,actor_id,
  CASE WHEN $3 THEN now()-interval '1 hour' ELSE now()+interval '1 hour' END,
  CASE WHEN $3 THEN NULL ELSE now() END FROM pangu.learning_source_consents
  WHERE namespace=$4 AND project_id=$5 AND source_id=$6 AND source_hash=$7`, uuid.NewString(), old.Hash, expired, f.scope.Namespace, f.scope.ProjectID, old.ID, current.Sources[0].Hash)
	id := uuid.NewString()
	if cause == "expired_uppercase" {
		old.ID, old.RunID, old.ProjectID = strings.ToUpper(old.ID), strings.ToUpper(old.RunID), strings.ToUpper(old.ProjectID)
	}
	sources, _ := json.Marshal([]experience.Source{old})
	f.exec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by)
  SELECT $1,namespace,project_id,$1,1,body_hash,$2,expires_at,created_by FROM pangu.experience_versions WHERE id=$3`, id, sources, current.ID)
	f.exec(t, `INSERT INTO pangu.experience_states(version_id,validity) VALUES($1,'candidate')`, id)
	f.exec(t, `INSERT INTO pangu.experience_bodies(version_id,content) SELECT $1,content FROM pangu.experience_bodies WHERE version_id=$2`, id, current.ID)
	return id
}

func TestRetentionUnknownConsentIsNotDeletionAuthority(t *testing.T) {
	f := newLearningFixture(t)
	current, _, checker := f.claim(t)
	source := current.Sources[0]
	source.ID = uuid.NewString()
	sources, _ := json.Marshal([]experience.Source{source})
	id := uuid.NewString()
	f.exec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by)
  SELECT $1,namespace,project_id,$1,1,body_hash,$2,expires_at,created_by FROM pangu.experience_versions WHERE id=$3`, id, sources, current.ID)
	f.exec(t, `INSERT INTO pangu.experience_states(version_id,validity) VALUES($1,'candidate')`, id)
	f.exec(t, `INSERT INTO pangu.experience_bodies(version_id,content) SELECT $1,content FROM pangu.experience_bodies WHERE version_id=$2`, id, current.ID)
	tx := f.transaction(t)
	out, err := PurgeExpiredTx(f.ctx, tx, f.scope.Namespace, f.scope.ProjectID, 10)
	if err != nil || out.Bodies != 0 || out.CheckerBodies != 0 {
		t.Fatal("missing consent inferred as expiry", out, err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if a, b := f.retentionCounts(t, id, checker.ID); a != 1 || b != 1 {
		t.Fatal("unknown retention deleted bytes", a, b)
	}
}
