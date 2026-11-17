//go:build integration

package approval

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/google/uuid"
)

func TestContinuationRejectsNewlySelectedMemoryOrDelivery(t *testing.T) {
	for _, kind := range []string{"new_version", "new_delivery_revision"} {
		t.Run(kind, func(t *testing.T) {
			f := newContinuationMemoryFixture(t)
			parent, child, input := f.continuePrepared(t, true, true, func() {
				if kind == "new_version" {
					f.additionalMemory(t)
					return
				}
				f.deliver(t, f.version.ID, 3)
			})
			if kind == "new_version" && len(input.Experiences) != 2 {
				t.Fatal("new memory was not selected")
			}
			if kind == "new_delivery_revision" && input.Experiences[0].Reference.DeliveryRevision != 3 {
				t.Fatal("new delivery was not selected")
			}
			if _, err := Request(f.ctx, f.actor.Namespace, child.ID, child.RunID, input); err == nil {
				t.Fatal("different trusted frozen selection inherited old approval")
			}
			f.noInheritedGate(t, parent, child.RunID)
		})
	}
}

func (f continuationMemoryFixture) additionalMemory(t *testing.T) {
	t.Helper()
	store := learning.NewStore()
	version, err := store.Create(f.ctx, f.scope, experience.Version{ProjectID: f.scope.ProjectID, Revision: 1, Content: f.version.Content, Sources: []experience.Source{f.source}, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := learning.CheckClaim(f.ctx, f.scope, version.ID, f.checker, f.intent.ID, f.intent.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Activate(f.ctx, f.scope, version.ID, receipt); err != nil {
		t.Fatal(err)
	}
	f.deliver(t, version.ID, 2)
}

func (f continuationMemoryFixture) deliver(t *testing.T, id string, revision int) {
	t.Helper()
	raw, err := json.Marshal(experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"submit", "plan"}, TrafficBasisPoints: 10000})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,evidence_id,actor_id) VALUES($1,$2,'serving',$3,$4,$5)`, id, revision, raw, uuid.NewString(), f.actor.ID)
}
