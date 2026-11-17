//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestEvolutionBusinessCoverageRequiresEachActualRequestNotJustSourceFamily(t *testing.T) {
	f := evolutionBusinessMemoryFixture(t)
	ctx := context.Background()
	f.request.PolicyRevision = 2
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	evolutionBusinessEntryRun(t, f, "needs_changes")
	next := evolutionNextFlowFixture(t, f, "")
	entryExec(t, `UPDATE pangu.evolution_bindings SET adopted_version=$3,revision=revision+1 WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID, next.ID)
	// Only observation eligibility is supplied; Observe must derive and store the real evidence.
	entryExec(t, `UPDATE pangu.evolution_intents SET state='observing',observing_at=clock_timestamp(),publication=$2 WHERE id=$1`, intent.ID, evolutionJSON(map[string]any{"flow_id": next.ID}))
	first := evolutionBusinessEntryRun(t, f, "accepted")
	if _, err = f.service.Observe(ctx, f.scope, intent.ID, 1, first); err != nil {
		t.Fatal(err)
	}
	complete, err := evolutionCoverageRead(t, f, intent.ID)
	if err != nil || !complete {
		t.Fatal("complete actual evidence did not cover candidate", complete, err)
	}
	second := evolutionBusinessEntryRun(t, f, "accepted")
	if _, err = f.service.Observe(ctx, f.scope, intent.ID, 1, second); !errors.Is(err, evolution.ErrConflict) {
		t.Fatal("duplicate baseline unexpectedly became independent evidence", err)
	}
	complete, err = evolutionCoverageRead(t, f, intent.ID)
	if err != nil || complete {
		t.Fatal("source family observation hid a distinct uncovered request", complete, err)
	}
}

func TestEvolutionBusinessCoverageHoldsAcceptanceFenceUntilCommit(t *testing.T) {
	f, id, request := evolutionCoverageFixture(t)
	ctx := context.Background()
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if _, err := (EvolutionEvidence{}).ObservationCoverageTx(ctx, tx, f.scope, id, 1); err != nil {
			return err
		}
		conn, err := database.Open(ctx)
		if err != nil {
			return err
		}
		defer conn.Close(ctx)
		other, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer other.Rollback(ctx)
		if _, err = other.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); err != nil {
			return err
		}
		_, err = submission.PrepareTx(ctx, other, f.scope.Namespace, "submit", uuid.NewString(), request.Snapshot)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
			t.Fatalf("acceptance raced past coverage transaction: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = submission.Prepare(ctx, f.scope.Namespace, "submit", uuid.NewString(), request.Snapshot); err != nil {
		t.Fatal("acceptance fence survived commit", err)
	}
}
