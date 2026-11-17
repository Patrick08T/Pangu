//go:build integration

package evolution

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// These methods model the trusted EvidenceReader cohort contract only. Actual
// accepted-request enumeration is independently tested by the scheduler backend.
func (r rolloutEvidence) ObservationCoverageTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int) (bool, error) {
	ids := make([]string, 0, len(r.samples))
	for id := range r.samples {
		ids = append(ids, id)
	}
	return rolloutFixtureCoverageTx(ctx, tx, scope, id, generation, ids)
}

func (r rolloutRevalidationReader) ObservationCoverageTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int) (bool, error) {
	var raw []byte
	if scope.ActorID != r.actor || id != r.id {
		return false, ErrForbidden
	}
	err := tx.QueryRow(ctx, `SELECT coalesce(request->'_testObservations','{}'::jsonb) FROM pangu.evolution_intents WHERE id=$1 AND namespace=$2 AND project_id=$3`, id, scope.Namespace, scope.ProjectID).Scan(&raw)
	if err != nil {
		return false, err
	}
	var sources map[string]rolloutRevalidationSource
	if err = json.Unmarshal(raw, &sources); err != nil {
		return false, err
	}
	ids := make([]string, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	return rolloutFixtureCoverageTx(ctx, tx, scope, id, generation, ids)
}

func rolloutFixtureCoverageTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int, ids []string) (bool, error) {
	var count int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM pangu.evolution_observations o JOIN pangu.evolution_intents i ON i.id=o.intent_id WHERE i.namespace=$1 AND i.project_id=$2 AND o.intent_id=$3 AND o.generation=$4 AND o.run_id=ANY($5::uuid[])`, scope.Namespace, scope.ProjectID, id, generation, ids).Scan(&count)
	return count == len(ids), err
}

// Deliberately does not expose ObservationCoverageTx.
type rolloutNoCoverageReader struct{ source rolloutEvidence }

func (r rolloutNoCoverageReader) ReadExecutionTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (MachineResult, error) {
	return r.source.ReadExecutionTx(ctx, tx, scope, id)
}
func (r rolloutNoCoverageReader) ReadObservationTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (BusinessEvidence, error) {
	return r.source.ReadObservationTx(ctx, tx, scope, id)
}

type rolloutCoverageReader struct {
	rolloutEvidence
	complete bool
	problem  string
	scope    Scope
	intent   string
}

func (r rolloutCoverageReader) ObservationCoverageTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int) (bool, error) {
	if scope != r.scope || id != r.intent || generation != 1 {
		return false, ErrForbidden
	}
	switch r.problem {
	case "sql":
		_, err := tx.Exec(ctx, `SELECT 1/0`)
		return false, err
	case "cancelled":
		return false, context.Canceled
	case "deadline":
		return false, context.DeadlineExceeded
	}
	if !r.complete {
		return false, nil
	}
	return r.rolloutEvidence.ObservationCoverageTx(ctx, tx, scope, id, generation)
}

func rolloutCoverageStarted(t *testing.T, f rolloutFixture) time.Time {
	t.Helper()
	var started time.Time
	err := transaction(context.Background(), func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT observing_at FROM pangu.evolution_intents WHERE id=$1`, f.id).Scan(&started)
	})
	if err != nil {
		t.Fatal(err)
	}
	return started
}

func rolloutCoverageSeed(t *testing.T, f *rolloutFixture) rolloutEvidence {
	t.Helper()
	f.observing(t)
	one, two := f.sample(), f.sample()
	r := rolloutEvidence{map[string]BusinessEvidence{one.RunID: one, two.RunID: two}}
	f.service.Evidence = r
	for _, sample := range []BusinessEvidence{one, two} {
		if _, err := f.service.Observe(context.Background(), f.scope, f.id, 1, sample.RunID); err != nil {
			t.Fatal(err)
		}
	}
	rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '2 seconds' WHERE id=$1`, f.id)
	return r
}
