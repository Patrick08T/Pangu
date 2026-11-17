//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func evolutionCoverageFixture(t *testing.T) (evolutionFixture, string, *submission.Record) {
	t.Helper()
	ctx := context.Background()
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	// Storage eligibility only: no passing decision, score, observation or result is fabricated.
	entryExec(t, `UPDATE pangu.evolution_intents SET state='observing',observing_at=clock_timestamp(),publication=$2 WHERE id=$1`, intent.ID, evolutionJSON(map[string]any{"flow_id": f.request.Payload.FlowVersion}))
	d, err := catalog.Compile(ctx, f.scope.Namespace, f.request.Payload.FlowVersion, f.scope.ProjectID, "JSON coverage candidate")
	if err != nil {
		t.Fatal(err)
	}
	record, err := submission.Prepare(ctx, f.scope.Namespace, "submit", uuid.NewString(), submission.Snapshot{Purpose: "business", CreatedBy: f.scope.ActorID, Workflow: *d})
	if err != nil {
		t.Fatal(err)
	}
	return f, intent.ID, record
}

func evolutionCoverageRead(t *testing.T, f evolutionFixture, id string) (bool, error) {
	t.Helper()
	var complete bool
	err := evolutionTransaction(context.Background(), func(tx pgx.Tx) error {
		var err error
		complete, err = (EvolutionEvidence{}).ObservationCoverageTx(context.Background(), tx, f.scope, id, 1)
		return err
	})
	return complete, err
}

func TestEvolutionBusinessCoverageNeverDropsUnknownCandidateRequests(t *testing.T) {
	for _, name := range []string{"pending", "failed", "cancelled", "purged", "snapshot_missing", "role_only"} {
		t.Run(name, func(t *testing.T) {
			f, id, request := evolutionCoverageFixture(t)
			switch name {
			case "failed", "cancelled":
				run := uuid.NewString()
				if _, err := submission.Dispatch(context.Background(), f.scope.Namespace, request.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
					t.Fatal(err)
				}
				entryExec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,clock_timestamp(),$3)`, run, f.scope.Namespace, evolutionJSON(map[string]any{"project_id": f.scope.ProjectID, "status": name, "stop_confirmed": true}))
			case "purged":
				entryExec(t, `UPDATE pangu.submissions SET purged_at=now(),snapshot='{}' WHERE id=$1`, request.ID)
			case "snapshot_missing":
				entryExec(t, `UPDATE pangu.submissions SET snapshot='{}' WHERE id=$1`, request.ID)
			case "role_only":
				entryExec(t, `UPDATE pangu.evolution_intents SET publication=$2 WHERE id=$1`, id, evolutionJSON(map[string]any{"agent_id": request.Snapshot.Workflow.Nodes[0].AgentVersion}))
			}
			complete, err := evolutionCoverageRead(t, f, id)
			if err != nil || complete {
				t.Fatal("unknown candidate silently disappeared from stable cohort", name, complete, err)
			}
		})
	}
}

func TestEvolutionBusinessCoveragePreservesSQLFailures(t *testing.T) {
	f, id, _ := evolutionCoverageFixture(t)
	err := evolutionTransaction(context.Background(), func(tx pgx.Tx) error {
		if err := tx.Rollback(context.Background()); err != nil {
			return err
		}
		complete, err := (EvolutionEvidence{}).ObservationCoverageTx(context.Background(), tx, f.scope, id, 1)
		if complete || !errors.Is(err, pgx.ErrTxClosed) {
			t.Fatalf("closed transaction became complete/empty cohort: %v %v", complete, err)
		}
		return err
	})
	if !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal(err)
	}
}
