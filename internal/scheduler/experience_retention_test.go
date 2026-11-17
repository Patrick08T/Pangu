//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestExperienceRetentionAuthoritativeCutoffs(t *testing.T) {
	cases := []struct {
		name  string
		ttl   int
		setup func(*testing.T, retentionFixture)
		want  bool
	}{
		{"frozen_expiry", 3600, func(*testing.T, retentionFixture) {}, true},
		{"not_due", 14400, func(*testing.T, retentionFixture) {}, false},
		{"new_longer_cannot_extend", 3600, func(t *testing.T, f retentionFixture) { f.policy(t, 2, 86400, true) }, true},
		{"new_shorter_wins", 14400, func(t *testing.T, f retentionFixture) { f.policy(t, 2, 3600, true) }, true},
		{"disabled_is_not_erasure", 14400, func(t *testing.T, f retentionFixture) { f.policy(t, 2, 14400, false) }, false},
		{"case_expired", 14400, func(t *testing.T, f retentionFixture) {
			f.exec(t, `UPDATE pangu.evaluation_cases SET expires_at=now()-interval '1 second' WHERE id=$1`, f.caseID)
		}, true},
		{"case_withdrawn", 14400, func(t *testing.T, f retentionFixture) {
			f.exec(t, `UPDATE pangu.evaluation_cases SET archived=true WHERE id=$1`, f.caseID)
		}, true},
		{"source_purged", 14400, func(t *testing.T, f retentionFixture) { f.exec(t, `SELECT pangu.purge_run_payloads($1)`, f.source) }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRetentionFixture(t, c.ttl)
			c.setup(t, f)
			if err := reconcileExperienceRetention(context.Background(), f.ns); err != nil {
				t.Fatal(err)
			}
			if got := f.erased(t); got != c.want {
				t.Fatalf("erased=%v want=%v", got, c.want)
			}
		})
	}
}

func TestExperienceRetentionRequiresStoppedAndUnlocked(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T, retentionFixture)
	}{
		{"unconfirmed", func(t *testing.T, f retentionFixture) {
			f.exec(t, `UPDATE pangu.evolution_case_runtime SET stop_confirmed=false WHERE request_id=$1`, f.request)
		}},
		{"running", func(t *testing.T, f retentionFixture) {
			f.exec(t, `UPDATE pangu.evolution_case_runtime SET stop_confirmed=false,status='running',finished_at=NULL WHERE request_id=$1`, f.request)
		}},
		{"active_attempt", func(t *testing.T, f retentionFixture) {
			f.exec(t, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt) VALUES($1,$2,'answer',1)`, f.run, uuid.NewString())
		}},
		{"runtime_lock", func(t *testing.T, f retentionFixture) {
			f.exec(t, `SELECT pg_advisory_lock(hashtextextended($1,0))`, "evolution-case:"+f.request)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRetentionFixture(t)
			c.setup(t, f)
			if err := reconcileExperienceRetention(context.Background(), f.ns); err != nil {
				t.Fatal(err)
			}
			if f.erased(t) {
				t.Fatal("unsafe evaluator was erased")
			}
		})
	}
}

func TestExperienceRetentionUnknownRuntimeAndCancellation(t *testing.T) {
	f := newRetentionFixture(t)
	f.exec(t, `DELETE FROM pangu.evolution_case_runtime WHERE request_id=$1`, f.request)
	if err := reconcileExperienceRetention(context.Background(), f.ns); err != nil {
		t.Fatal(err)
	}
	var present bool
	if err := f.conn.QueryRow(context.Background(), `SELECT plan IS NOT NULL FROM pangu.evolution_case_plans WHERE request_id=$1`, f.request).Scan(&present); err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("missing runtime is not proof of stopped execution")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := reconcileExperienceRetention(ctx, f.ns); err == nil {
		t.Fatal("cancelled scan must propagate error")
	}
}

func TestExperienceRetentionMalformedAuthorityPropagates(t *testing.T) {
	f := newRetentionFixture(t)
	f.exec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,2,$3,'{"observation":{"retention_seconds":"unknown"}}')`, f.ns, f.project, f.actor)
	if err := reconcileExperienceRetention(context.Background(), f.ns); err == nil {
		t.Fatal("malformed authority must not become expiry")
	}
	if f.erased(t) {
		t.Fatal("unknown authority erased private data")
	}
}
