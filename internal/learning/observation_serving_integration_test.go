//go:build integration

package learning

import (
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestMemoryObservationAlsoReadsActualServingConsumption(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	policy, _ := json.Marshal(experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"business"}, TrafficBasisPoints: 10000})
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,3,'serving',$2,$3)`, version.ID, policy, f.actor.ID)
	version, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	source := f.observationRun(t, version, "accepted")
	observation, err := ObserveMemory(f.ctx, f.scope, version.ID, source.RunID)
	if err != nil || !observation.Accepted || observation.DeliveryRevision != 3 {
		t.Fatal("real serving observation rejected", observation, err)
	}
}
