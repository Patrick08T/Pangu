//go:build integration

package learning

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func (f learningFixture) retentionExpiredCopy(t *testing.T, version experience.Version, checker Checker) (string, string) {
	t.Helper()
	id, checkerID := uuid.NewString(), uuid.NewString()
	f.exec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by)
  SELECT $1,namespace,project_id,$1,1,body_hash,sources,now()-interval '1 hour',created_by FROM pangu.experience_versions WHERE id=$2`, id, version.ID)
	f.exec(t, `INSERT INTO pangu.experience_states(version_id,validity) VALUES($1,'candidate')`, id)
	f.exec(t, `INSERT INTO pangu.experience_bodies(version_id,content) SELECT $1,content FROM pangu.experience_bodies WHERE version_id=$2`, id, version.ID)
	f.exec(t, `INSERT INTO pangu.learning_checkers(id,namespace,project_id,content_hash,config_hash,actor_id,expires_at)
  SELECT $1,namespace,project_id,content_hash,config_hash,actor_id,now()-interval '1 hour' FROM pangu.learning_checkers WHERE id=$2`, checkerID, checker.ID)
	f.exec(t, `INSERT INTO pangu.learning_checker_bodies(checker_id,body) SELECT $1,body FROM pangu.learning_checker_bodies WHERE checker_id=$2`, checkerID, checker.ID)
	return id, checkerID
}

func (f learningFixture) retentionCounts(t *testing.T, versionID, checkerID string) (int, int) {
	t.Helper()
	tx := f.transaction(t)
	var versions, checkers int
	if err := tx.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM pangu.experience_bodies WHERE version_id=$1),
  (SELECT count(*) FROM pangu.learning_checker_bodies WHERE checker_id=$2)`, versionID, checkerID).Scan(&versions, &checkers); err != nil {
		t.Fatal(err)
	}
	return versions, checkers
}
