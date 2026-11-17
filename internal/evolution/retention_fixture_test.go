//go:build integration

package evolution

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type retentionFixture struct {
	rolloutFixture
	experiment, memory, requestHash string
}

func newRetentionFixture(t *testing.T, reason string) retentionFixture {
	t.Helper()
	return attachRetentionFixture(t, newRolloutFixture(t), reason)
}

func attachRetentionFixture(t *testing.T, base rolloutFixture, reason string) retentionFixture {
	t.Helper()
	f := retentionFixture{rolloutFixture: base, memory: uuid.NewString()}
	err := transaction(context.Background(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(context.Background(), `SELECT source_experiment::text,(SELECT request_hash FROM pangu.evolution_intents WHERE id=$2) FROM pangu.catalog_versions WHERE id=$1`, f.candidate, f.id).Scan(&f.experiment, &f.requestHash); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.addSource(t, reason)
	return f
}

func (f retentionFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if err := transaction(context.Background(), func(tx pgx.Tx) error { _, err := tx.Exec(context.Background(), sql, args...); return err }); err != nil {
		t.Fatal(err)
	}
}

func (f retentionFixture) addSource(t *testing.T, reason string) {
	t.Helper()
	source := experience.Source{ID: uuid.NewString(), Kind: "report", RunID: uuid.NewString(), NodeID: "answer", ProjectID: f.scope.ProjectID, Hash: hashText("source")}
	expiry := "1 day"
	if reason == "memory_expired" {
		expiry = "-1 hour"
	}
	f.exec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by) VALUES($1,$2,$3,$4,1,$5,$6,clock_timestamp()+$7::interval,$8)`, f.memory, f.scope.Namespace, f.scope.ProjectID, uuid.NewString(), hashText(f.memory), jsonBytes([]experience.Source{source}), expiry, f.scope.ActorID)
	f.exec(t, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{request,memory_version_ids}',$2) WHERE id=$1`, f.id, jsonBytes([]string{f.memory}))
	if reason == "missing_consent" {
		return
	}
	expiry = "1 day"
	if reason == "source_expired" {
		expiry = "-1 hour"
	}
	if reason == "wrong_hash" {
		source.Hash = hashText("different revision")
	}
	f.exec(t, `INSERT INTO pangu.learning_source_consents(id,namespace,project_id,source_id,source_kind,run_id,node_id,source_hash,actor_id,expires_at,revoked_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,clock_timestamp()+$10::interval,CASE WHEN $11 THEN clock_timestamp() ELSE NULL END)`, uuid.NewString(), f.scope.Namespace, f.scope.ProjectID, source.ID, source.Kind, source.RunID, source.NodeID, source.Hash, f.scope.ActorID, expiry, reason == "withdrawn" || reason == "wrong_hash")
	if reason == "policy_expired" {
		f.exec(t, `UPDATE pangu.evolution_intents SET created_at=clock_timestamp()-interval '100 days' WHERE id=$1`, f.id)
	}
}

func (f retentionFixture) retained(t *testing.T) (bool, bool, string, int) {
	t.Helper()
	var candidate, version bool
	var state string
	var claims int
	err := transaction(context.Background(), func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT c.payload IS NOT NULL,b.spec IS NOT NULL,s.state,(SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1) FROM pangu.catalog_experiment_candidates c JOIN pangu.catalog_version_bodies b ON b.version_id=$2 JOIN pangu.catalog_version_state s ON s.version_id=b.version_id WHERE c.experiment_id=$3`, f.id, f.candidate, f.experiment).Scan(&candidate, &version, &state, &claims)
	})
	if err != nil {
		t.Fatal(err)
	}
	return candidate, version, state, claims
}

func (f retentionFixture) reconcile(t *testing.T, guard RetentionGuard) (RetentionResult, error) {
	t.Helper()
	scope := f.scope
	scope.ActorID = ""
	return f.service.ReconcileRetention(context.Background(), scope, f.id, 1, guard)
}

func retentionAllowed(context.Context, pgx.Tx, Scope, RetentionTarget, string) (bool, error) {
	return true, nil
}
