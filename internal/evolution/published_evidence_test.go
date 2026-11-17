//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/jackc/pgx/v5"
)

// All publication authority comes through Reserve/Prepare/Record/Decide/Promote;
// only execution-output and business-evidence boundaries use existing test readers.
func publishedEvidenceFixture(t *testing.T) rolloutFixture {
	t.Helper()
	f, intent := completedDecisionFixture(t)
	decision, err := f.service.Decide(context.Background(), f.scope, intent.ID, intent.Generation)
	assertPassedDecision(t, decision, err)
	intent, err = f.service.Promote(context.Background(), f.scope, intent.ID, intent.Generation)
	if err != nil || intent.Publication == nil || intent.State != "switching" {
		t.Fatal("real promotion failed", intent, err)
	}
	return rolloutFixture{reservationFixture: f, id: intent.ID, candidate: intent.Publication.AgentID}
}

func TestPublishedEvidenceReconcileRejectsCurrentSourceInvalidity(t *testing.T) {
	for _, kind := range []string{"archived", "expired", "purged", "invalidated"} {
		t.Run(kind, func(t *testing.T) {
			f := publishedEvidenceFixture(t)
			invalidatePublishedEvidence(t, f, kind)
			got, err := f.service.Reconcile(context.Background(), f.scope, f.id, 1)
			if err != nil || got.State != "insufficient" {
				t.Fatal("invalid offline evidence still adopted", got, err)
			}
			b, claims := f.binding(t)
			if b.AdoptedVersion != f.request.BaselineVersion || claims != 0 {
				t.Fatal("compensation not confirmed", b, claims)
			}
			assertPublishedBody(t, f, "revoked")
		})
	}
}

func invalidatePublishedEvidence(t *testing.T, f rolloutFixture, kind string) {
	t.Helper()
	c := f.suite.Cases[0]
	switch kind {
	case "archived":
		rolloutExec(t, `UPDATE pangu.evaluation_cases SET archived=true WHERE id=$1`, c.ID)
	case "expired":
		rolloutExec(t, `UPDATE pangu.evaluation_cases SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, c.ID)
	case "purged":
		rolloutExec(t, `INSERT INTO pangu.retention_records(namespace,project_id,run_id,worker_id,status) VALUES($1,$2,$3,'published-evidence-test','purged')`, f.scope.Namespace, f.scope.ProjectID, c.SourceRunID)
	case "invalidated":
		err := transaction(context.Background(), func(tx pgx.Tx) error {
			intent, err := readIntentTx(context.Background(), tx, f.scope, f.id)
			if err != nil {
				return err
			}
			return evaluation.InvalidateExperimentTx(context.Background(), tx, f.scope.Namespace, intent.ExperimentID, 1)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertPublishedBody(t *testing.T, f rolloutFixture, state string) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var actual string
	var body bool
	err = conn.QueryRow(context.Background(), `SELECT s.state,b.spec IS NOT NULL FROM pangu.catalog_version_state s JOIN pangu.catalog_version_bodies b USING(version_id) WHERE version_id=$1`, f.candidate).Scan(&actual, &body)
	if err != nil || actual != state || !body {
		t.Fatal("eligibility was confused with body erasure", actual, body, err)
	}
}

func TestPublishedEvidenceDirectAdoptAndCompletionRejectInvalidSource(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "prepare", true: "complete"}[complete], func(t *testing.T) {
			f := publishedEvidenceFixture(t)
			ctx := context.Background()
			if complete {
				if _, err := f.service.PrepareSwitch(ctx, f.scope, f.id, 1, "workbench:main", "adopt"); err != nil {
					t.Fatal(err)
				}
			}
			invalidatePublishedEvidence(t, f, "archived")
			var err error
			if complete {
				_, err = f.service.CompleteSwitch(ctx, f.scope, f.id, 1, "workbench:main", "adopt")
			} else {
				_, err = f.service.PrepareSwitch(ctx, f.scope, f.id, 1, "workbench:main", "adopt")
			}
			if err == nil {
				t.Fatal("direct adopt accepted invalid offline evidence")
			}
			_, claims := f.binding(t)
			if claims != 2 {
				t.Fatal("rejection prematurely released claims", claims)
			}
			assertPublishedBody(t, f, "revoked")
		})
	}
}

func publishedGoodObservations(t *testing.T, f *rolloutFixture) {
	t.Helper()
	f.observing(t)
	one, two := f.sample(), f.sample()
	f.service.Evidence = rolloutEvidence{map[string]BusinessEvidence{one.RunID: one, two.RunID: two}}
	for _, id := range []string{one.RunID, two.RunID} {
		if _, err := f.service.Observe(context.Background(), f.scope, f.id, 1, id); err != nil {
			t.Fatal(err)
		}
	}
	rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '2 seconds' WHERE id=$1`, f.id)
}

func TestPublishedEvidenceAdvanceCannotReplaceOfflineProofWithBusinessSamples(t *testing.T) {
	f := publishedEvidenceFixture(t)
	publishedGoodObservations(t, &f)
	invalidatePublishedEvidence(t, f, "purged")
	got, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
	if err != nil || got.State != "rolling_back" {
		t.Fatal("invalid offline evidence stabilized", got, err)
	}
	_, claims := f.binding(t)
	if claims != 2 {
		t.Fatal("advance released claims before compensation", claims)
	}
	assertPublishedBody(t, f, "revoked")
	got, err = f.service.Reconcile(context.Background(), f.scope, f.id, 1)
	if err != nil || got.State != "insufficient" {
		t.Fatal("offline invalidity was not insufficient", got, err)
	}
}

func TestPublishedEvidenceEvaluatorDisabledAloneDoesNotRevokeOrErase(t *testing.T) {
	f := publishedEvidenceFixture(t)
	_, err := f.service.RegisterEnvironment(context.Background(), f.scope, Environment{ID: f.request.EnvironmentID, ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", Enabled: false, ConnectionVersion: f.request.Payload.Agent.ConnectionVersion}, 1)
	if err != nil {
		t.Fatal(err)
	}
	publishedGoodObservations(t, &f)
	got, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
	if err != nil || got.State != "stable" {
		t.Fatal("executor pause revoked serving eligibility", got, err)
	}
	assertPublishedBody(t, f, "stable")
}

func TestPublishedEvidenceUnknownSourceReadRetainsClaimsAndState(t *testing.T) {
	f := publishedEvidenceFixture(t)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SELECT 1 FROM pangu.evaluation_cases WHERE id=$1 FOR UPDATE`, f.suite.Cases[0].ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = f.service.Reconcile(ctx, f.scope, f.id, 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unknown source read became decision", err)
	}
	b, claims := f.binding(t)
	if b.Revision != 1 || claims != 2 {
		t.Fatal("unknown evidence changed targets", b, claims)
	}
	assertPublishedBody(t, f, "observing")
}
