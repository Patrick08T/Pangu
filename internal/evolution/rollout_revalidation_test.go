//go:build integration

package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 只替代可信证据边界；来源快照、观测、晋级和补偿均使用真实数据库事务。
type rolloutRevalidationSource struct {
	Evidence BusinessEvidence `json:"evidence"`
	Problem  string           `json:"problem,omitempty"`
}

type rolloutRevalidationReader struct{ id, actor string }

func (r rolloutRevalidationReader) ReadExecutionTx(context.Context, pgx.Tx, Scope, string) (MachineResult, error) {
	return MachineResult{}, ErrChecker
}

func (r rolloutRevalidationReader) ReadObservationTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (BusinessEvidence, error) {
	var raw []byte
	var source rolloutRevalidationSource
	if scope.ActorID != r.actor {
		return source.Evidence, ErrForbidden
	}
	err := tx.QueryRow(ctx, `SELECT request#>ARRAY['_testObservations',$2] FROM pangu.evolution_intents WHERE id=$1 AND namespace=$3 AND project_id=$4 FOR SHARE`, r.id, id, scope.Namespace, scope.ProjectID).Scan(&raw)
	if err != nil {
		return source.Evidence, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return source.Evidence, ErrChecker
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		return source.Evidence, err
	}
	switch source.Problem {
	case "expired":
		err = ErrChecker
	case "deleted":
		err = ErrNotFound
	case "revoked":
		err = ErrForbidden
	case "conflict":
		err = ErrConflict
	case "invalidated":
		err = experience.ErrInvalidated
	case "cancelled":
		err = context.Canceled
	case "deadline":
		err = context.DeadlineExceeded
	case "sql":
		_, err = tx.Exec(ctx, `SELECT 1/0`)
	}
	return source.Evidence, err
}

func rolloutRevalidationPut(t *testing.T, f rolloutFixture, id string, source rolloutRevalidationSource) {
	t.Helper()
	rolloutExec(t, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{_testObservations}',coalesce(request->'_testObservations','{}'::jsonb)||jsonb_build_object($2::text,$3::jsonb)) WHERE id=$1`, f.id, id, jsonBytes(source))
}

func rolloutRevalidationSeed(t *testing.T, f *rolloutFixture, key string, score float64) []BusinessEvidence {
	t.Helper()
	f.service.Evidence = rolloutRevalidationReader{f.id, f.scope.ActorID}
	cohort := make([]BusinessEvidence, 3)
	for i := range cohort {
		cohort[i] = f.sample()
		cohort[i].EntryKey, cohort[i].CandidateScore = key, score
		rolloutRevalidationPut(t, *f, cohort[i].RunID, rolloutRevalidationSource{Evidence: cohort[i]})
		if _, err := f.service.Observe(context.Background(), f.scope, f.id, 1, cohort[i].RunID); err != nil {
			t.Fatal(err)
		}
	}
	rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '2 seconds' WHERE id=$1`, f.id)
	return cohort
}

func rolloutRevalidationState(t *testing.T, f rolloutFixture, state, versionState string, samples, claims int) {
	t.Helper()
	err := transaction(context.Background(), func(tx pgx.Tx) error {
		var actualState, actualVersion string
		var count int
		if err := tx.QueryRow(context.Background(), `SELECT i.state,s.state,(SELECT count(*) FROM pangu.evolution_observations o WHERE o.intent_id=i.id) FROM pangu.evolution_intents i JOIN pangu.catalog_version_state s ON s.version_id=$2 WHERE i.id=$1`, f.id, f.candidate).Scan(&actualState, &actualVersion, &count); err != nil {
			return err
		}
		if actualState != state || actualVersion != versionState || count != samples {
			t.Errorf("unexpected persisted state/cohort: %s %s %d", actualState, actualVersion, count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, count := f.binding(t)
	if count != claims || (claims > 0 && binding.StableVersion != f.request.BaselineVersion) {
		t.Fatal("released claims or advanced stable before confirmation", binding, count)
	}
}

func TestRolloutRevalidationRejectsChangedEvidence(t *testing.T) {
	changes := map[string]func(*BusinessEvidence){
		"source_hash":      func(e *BusinessEvidence) { e.SourceHash = hashText("revised feedback") },
		"run":              func(e *BusinessEvidence) { e.RunID = uuid.NewString() },
		"baseline_run":     func(e *BusinessEvidence) { e.BaselineRunID = uuid.NewString() },
		"entry":            func(e *BusinessEvidence) { e.EntryKey = "workbench:other" },
		"version":          func(e *BusinessEvidence) { e.VersionID = uuid.NewString() },
		"baseline_version": func(e *BusinessEvidence) { e.BaselineVersionID = uuid.NewString() },
		"purpose":          func(e *BusinessEvidence) { e.Purpose = "evaluation" },
		"terminal":         func(e *BusinessEvidence) { e.Terminal = false },
		"stop":             func(e *BusinessEvidence) { e.StopConfirmed = false },
		"candidate_score":  func(e *BusinessEvidence) { e.CandidateScore = 0 },
		"baseline_score":   func(e *BusinessEvidence) { e.BaselineScore = 0 },
		"hard_failure":     func(e *BusinessEvidence) { e.HardFailure = true },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			f := newRolloutFixture(t)
			f.observing(t)
			cohort := rolloutRevalidationSeed(t, &f, "workbench:main", 1)
			changed := cohort[2] // 前两条已满足样本门槛，不能忽略队列末尾的失效证据。
			change(&changed)
			rolloutRevalidationPut(t, f, cohort[2].RunID, rolloutRevalidationSource{Evidence: changed})
			intent, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
			if err != nil || intent.State != "rolling_back" {
				t.Fatal("changed evidence was promoted", intent, err)
			}
			rolloutRevalidationState(t, f, "rolling_back", "revoked", 3, 2)
			intent, err = f.service.Reconcile(context.Background(), f.scope, f.id, 1)
			if err != nil || intent.State != "insufficient" {
				t.Fatal("invalid evidence treated as measured regression", intent, err)
			}
		})
	}
}

func TestRolloutRevalidationKnownInvalidSources(t *testing.T) {
	for _, problem := range []string{"expired", "deleted", "revoked", "conflict", "invalidated", "purged"} {
		t.Run(problem, func(t *testing.T) {
			f := newRolloutFixture(t)
			f.observing(t)
			cohort := rolloutRevalidationSeed(t, &f, "workbench:main", 1)
			rolloutRevalidationPut(t, f, cohort[0].RunID, rolloutRevalidationSource{Evidence: cohort[0], Problem: problem})
			if problem == "purged" {
				rolloutExec(t, `UPDATE pangu.evolution_intents SET request=request#-ARRAY['_testObservations',$2] WHERE id=$1`, f.id, cohort[0].RunID)
			}
			intent, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
			if err != nil || intent.State != "rolling_back" {
				t.Fatal("unavailable evidence was promoted", intent, err)
			}
			rolloutRevalidationState(t, f, "rolling_back", "revoked", 3, 2)
		})
	}
}

func TestRolloutRevalidationUnknownFailureDoesNotInvalidate(t *testing.T) {
	for _, problem := range []string{"sql", "cancelled", "deadline"} {
		t.Run(problem, func(t *testing.T) {
			f := newRolloutFixture(t)
			f.observing(t)
			cohort := rolloutRevalidationSeed(t, &f, "workbench:main", 0)
			rolloutRevalidationPut(t, f, cohort[0].RunID, rolloutRevalidationSource{Evidence: cohort[0], Problem: "expired"})
			rolloutRevalidationPut(t, f, cohort[2].RunID, rolloutRevalidationSource{Evidence: cohort[2], Problem: problem})
			_, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
			var sql *pgconn.PgError
			if (problem == "sql" && (!errors.As(err, &sql) || sql.Code != "22012")) || (problem == "cancelled" && !errors.Is(err, context.Canceled)) || (problem == "deadline" && !errors.Is(err, context.DeadlineExceeded)) {
				t.Fatal("unknown error was converted into a decision", problem, err)
			}
			rolloutRevalidationState(t, f, "observing", "observing", 3, 2)
		})
	}
}

func TestRolloutRevalidationUnchangedCohortCanStabilize(t *testing.T) {
	f := newRolloutFixture(t)
	f.observing(t)
	rolloutRevalidationSeed(t, &f, "workbench:main", 1)
	intent, err := f.service.Advance(context.Background(), f.scope, f.id, 1)
	if err != nil || intent.State != "stable" {
		t.Fatal("unchanged trusted evidence could not stabilize", intent, err)
	}
	rolloutRevalidationState(t, f, "stable", "stable", 3, 0)
}

func TestRolloutRevalidationMissingReaderCannotUseCachedSamples(t *testing.T) {
	f := newRolloutFixture(t)
	f.observing(t)
	rolloutRevalidationSeed(t, &f, "workbench:main", 1)
	f.service.Evidence = nil
	if _, err := f.service.Advance(context.Background(), f.scope, f.id, 1); !errors.Is(err, ErrChecker) {
		t.Fatal("missing checker used cached evidence", err)
	}
	rolloutRevalidationState(t, f, "observing", "observing", 3, 2)
}
