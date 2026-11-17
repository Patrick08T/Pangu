//go:build integration

package learning

import (
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func (f learningFixture) feedback(t *testing.T, report experience.Source, decision string) experience.Source {
	t.Helper()
	var task string
	if err := f.transaction(t).QueryRow(f.ctx, `SELECT task_id FROM pangu.submissions WHERE run_id=$1`, report.RunID).Scan(&task); err != nil {
		t.Fatal(err)
	}
	body := FeedbackPayload{ID: uuid.NewString(), RunID: report.RunID, TaskID: task, ActorID: f.actor.ID,
		Revision: 1, SubjectHash: digest([]byte("subject")), Decision: decision, FirstDecision: decision, Reason: "human review"}
	f.exec(t, `INSERT INTO pangu.feedback(id,namespace,project_id,run_id,task_id,actor_id,revision,subject_hash,decision,first_decision,reason)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, body.ID, f.scope.Namespace, f.scope.ProjectID, body.RunID, body.TaskID,
		body.ActorID, body.Revision, body.SubjectHash, body.Decision, body.FirstDecision, body.Reason)
	return experience.Source{ID: body.ID, Kind: "feedback", RunID: body.RunID, ProjectID: f.scope.ProjectID, Hash: jsonHash(body)}
}

func TestLearningFeedbackHashesOriginalContentAndRevision(t *testing.T) {
	f := newLearningFixture(t)
	f.policy(t)
	source := f.feedback(t, f.report(t), "accepted")
	if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSource(f.ctx, f.scope, source); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE pangu.feedback SET reason='changed',revision=revision+1 WHERE id=$1`, source.ID)
	if _, err := ReadSource(f.ctx, f.scope, source); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatal("changed original feedback remained valid", err)
	}
}

func TestLearningRejectsProtectedSourceAndWrongLogicalNode(t *testing.T) {
	f := newLearningFixture(t)
	f.policy(t)
	source := f.report(t)
	wrong := source
	wrong.NodeID = "implementation"
	if err := GrantSource(f.ctx, f.scope, wrong, time.Now().Add(time.Hour)); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatal("wrong node accepted", err)
	}
	pool := uuid.NewString()
	f.exec(t, `INSERT INTO pangu.evaluation_holdout_pools(id,namespace,project_id,holdout_hash,rules_hash,max_candidates)
	 VALUES($1,$2,$3,$4,$4,1)`, pool, f.scope.Namespace, f.scope.ProjectID, digest([]byte("holdout")))
	f.exec(t, `INSERT INTO pangu.evaluation_identity_allocations(namespace,project_id,identity_kind,identity_key,partition,pool_id)
	 VALUES($1,$2,'source',$3,'holdout',$4)`, f.scope.Namespace, f.scope.ProjectID, source.RunID, pool)
	if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("hidden source accepted", err)
	}
}

func TestLearningRevokedPolicyAndConsentOwnerLoseSourceAccess(t *testing.T) {
	for _, change := range []string{"policy", "owner"} {
		t.Run(change, func(t *testing.T) {
			f := newLearningFixture(t)
			policy := f.policy(t)
			source := f.report(t)
			if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if change == "policy" {
				policy.Enabled, policy.ReadEnabled, policy.AutoPublish, policy.Revoked = false, false, false, true
				if _, err := f.store.SavePolicy(f.ctx, f.scope, policy, policy.Revision); err != nil {
					t.Fatal(err)
				}
			} else {
				f.exec(t, `UPDATE pangu.memberships SET role='viewer' WHERE user_id=$1`, f.actor.ID)
			}
			if _, err := ReadSource(f.ctx, f.scope, source); !errors.Is(err, experience.ErrForbidden) {
				t.Fatal("withdrawn authority retained source access", err)
			}
		})
	}
}
