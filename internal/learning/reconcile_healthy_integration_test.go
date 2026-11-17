//go:build integration

package learning

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func TestMemoryServingMaintenanceUsesFixedCompleteWindows(t *testing.T) {
	f := newLearningFixture(t)
	version, experiment, _ := f.benefit(t)
	policy, _ := json.Marshal(experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"business"}, TrafficBasisPoints: 10000})
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id,evidence_id,created_at)
  VALUES($1,2,'serving',$2,$3,$4,now()-interval '120 seconds')`, version.ID, policy, f.actor.ID, experiment.ID)
	version, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		f.historicalObservation(t, version, i)
	}
	reader := func(context.Context, pgx.Tx, experience.Scope, string) (evaluation.ExperimentSpec, error) {
		return experiment.ExperimentSpec, nil
	}
	for i := 0; i < 2; i++ {
		paused, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, reader)
		if err != nil || paused {
			t.Fatal("healthy or not-yet-deadline window paused", paused, err)
		}
	}
	tx := f.transaction(t)
	defer tx.Rollback(f.ctx)
	var count int
	err = tx.QueryRow(f.ctx, `SELECT count(*) FROM pangu.learning_events WHERE subject_id=$1 AND action='memory_window_healthy'`, version.ID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatal("healthy window missing or reused for later empty window", count, err)
	}
}
