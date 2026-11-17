//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Removing the complete-cohort gate must turn these safe observations into the
// previous (incorrect) stable transition, despite two valid success samples.
func TestRolloutCoverageMissingCohortWaitsThenBecomesInsufficient(t *testing.T) {
	for _, missing := range []string{"uncovered", "missing_reader"} {
		t.Run(missing, func(t *testing.T) {
			f := newRolloutFixture(t)
			source := rolloutCoverageSeed(t, &f)
			f.service.Evidence = rolloutCoverageReader{rolloutEvidence: source, scope: f.scope, intent: f.id}
			if missing == "missing_reader" {
				f.service.Evidence = rolloutNoCoverageReader{source}
			}
			started := rolloutCoverageStarted(t, f)
			intent, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
			if err != nil || intent.State != "observing" {
				t.Fatal("incomplete accepted cohort stabilized before deadline", intent, err)
			}
			rolloutRevalidationState(t, f, "observing", "observing", 2, 2)
			if !rolloutCoverageStarted(t, f).Equal(started) {
				t.Fatal("coverage wait reset the original observation deadline")
			}
			rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '20 seconds' WHERE id=$1`, f.id)
			intent, err = f.service.Advance(context.Background(), f.scope, f.id, 1)
			if err != nil || intent.State != "rolling_back" {
				t.Fatal("incomplete cohort reset deadline or stabilized", intent, err)
			}
			rolloutRevalidationState(t, f, "rolling_back", "revoked", 2, 2)
			intent, err = f.service.Reconcile(context.Background(), f.scope, f.id, 1)
			if err != nil || intent.State != "insufficient" {
				t.Fatal("missing cohort lost bounded compensation", intent, err)
			}
			assertPublishedBody(t, f, "revoked")
		})
	}
}

func TestRolloutCoverageCompleteCohortAllowsStable(t *testing.T) {
	f := newRolloutFixture(t)
	source := rolloutCoverageSeed(t, &f)
	f.service.Evidence = rolloutCoverageReader{rolloutEvidence: source, complete: true, scope: f.scope, intent: f.id}
	scope := f.scope
	scope.ActorID = ""
	intent, err := f.service.Advance(context.Background(), scope, f.id, 1)
	if err != nil || intent.State != "stable" {
		t.Fatal("complete current cohort did not stabilize", intent, err)
	}
	b, claims := f.binding(t)
	if b.StableVersion != f.candidate || claims != 0 {
		t.Fatal("stable pointers or released claims wrong", b, claims)
	}
	assertPublishedBody(t, f, "stable")
}

func TestRolloutCoverageUnknownReadPreservesClaimsAndBody(t *testing.T) {
	for _, problem := range []string{"sql", "cancelled", "deadline"} {
		t.Run(problem, func(t *testing.T) {
			f := newRolloutFixture(t)
			source := rolloutCoverageSeed(t, &f)
			f.service.Evidence = rolloutCoverageReader{rolloutEvidence: source, problem: problem, scope: f.scope, intent: f.id}
			rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '20 seconds' WHERE id=$1`, f.id)
			_, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
			switch problem {
			case "sql":
				var sql *pgconn.PgError
				if !errors.As(err, &sql) || sql.Code != "22012" {
					t.Fatal("coverage SQL failure became safe absence", err)
				}
			case "cancelled":
				if !errors.Is(err, context.Canceled) {
					t.Fatal("lost cancelled coverage error", err)
				}
			case "deadline":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("lost coverage deadline error", err)
				}
			}
			rolloutRevalidationState(t, f, "observing", "observing", 2, 2)
			assertPublishedBody(t, f, "observing")
		})
	}
}

func TestRolloutCoverageNilEvidenceCannotUsePersistedSuccesses(t *testing.T) {
	f := newRolloutFixture(t)
	rolloutCoverageSeed(t, &f)
	f.service.Evidence = nil
	_, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
	if !errors.Is(err, ErrChecker) {
		t.Fatal("nil evidence trusted saved successes", err)
	}
	rolloutRevalidationState(t, f, "observing", "observing", 2, 2)
	assertPublishedBody(t, f, "observing")
}
