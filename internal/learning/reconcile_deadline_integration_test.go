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

func TestMemoryReconcilePausesExpiredIncompleteWindow(t *testing.T) {
	for _, mode := range []string{"canary", "serving"} {
		t.Run(mode, func(t *testing.T) {
			f := newLearningFixture(t)
			version, experiment, _ := f.benefit(t)
			policy := experience.DeliveryPolicy{Mode: mode, TaskScope: []string{"business"}, TrafficBasisPoints: 10000}
			data, _ := json.Marshal(policy)
			f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id,evidence_id,created_at)
   VALUES($1,2,$2,$3,$4,$5,now()-interval '7200 seconds')`, version.ID, mode, data, f.actor.ID, experiment.ID)
			reader := func(context.Context, pgx.Tx, experience.Scope, string) (evaluation.ExperimentSpec, error) {
				return experiment.ExperimentSpec, nil
			}
			paused, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, reader)
			if err != nil || !paused {
				t.Fatal("deadline with inadequate evidence not paused", paused, err)
			}
			assertNoFrozenBlock(t, f, version.ID)
		})
	}
}

func TestMemoryNamespaceMaintenancePersistsPerVersionPause(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	count, err := ReconcileMemory(context.Background(), f.scope.Namespace, nil)
	if err != nil || count != 1 {
		t.Fatal("namespace maintenance did not pause", count, err)
	}
	current, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil || current.Delivery != "paused" {
		t.Fatal(current, err)
	}
}
