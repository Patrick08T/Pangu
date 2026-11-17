//go:build integration

package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func TestQueryMemoryBenefitProjectsOnlySafeExperimentIdentity(t *testing.T) {
	f := newReservationFixture(t)
	memory := benefitMemory(t, &f)
	f.request.Kind = "memory_benefit"
	f.request.BenefitMemoryVersionID = memory.ID
	f.request.MemoryVersionIDs = []string{memory.ID}
	// This service freezes an unchanged baseline for memory-only experiments.
	f.request.Payload.Agent = workflow.Node{}
	intent, err := f.service.ReserveCandidate(context.Background(), f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	ctx := identity.WithActor(context.Background(), identity.Actor{ID: f.scope.ActorID, Namespace: f.scope.Namespace})
	summary, err := f.service.ReadIntentSummary(ctx, f.scope, intent.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(summary)
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value["kind"] != "memory_benefit" || value["experiment_id"] != intent.ExperimentID || value["benefit_memory_version_id"] != memory.ID {
		t.Fatalf("safe identity missing: %s", raw)
	}
	for _, key := range []string{"request", "payload", "suite", "suite_id", "receipts", "results"} {
		if _, ok := value[key]; ok {
			t.Fatal("private experiment data leaked", key)
		}
	}
}

func TestQueryScopesAndDisabledPolicyRemainAuditable(t *testing.T) {
	f := newGovernanceFixture(t)
	ctx := identity.WithActor(context.Background(), identity.Actor{ID: f.scope.ActorID, Namespace: f.scope.Namespace})
	page, err := (Service{}).ListIntents(ctx, f.scope, QueryPage{Limit: 10})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("empty project: %+v %v", page, err)
	}
	f.policy.Revision++
	f.policy.Enabled = false
	f.savePolicy(t)
	if _, err = (Service{}).ListEnvironments(ctx, f.scope, QueryPage{Limit: 10}); err != nil {
		t.Fatal("disabled governance must remain readable", err)
	}
	other := f.scope
	other.ProjectID = uuid.NewString()
	if _, err = (Service{}).ListIntents(ctx, other, QueryPage{Limit: 10}); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign scope", err)
	}
	for _, page := range []QueryPage{{Limit: 0}, {Limit: 101}, {Limit: 10, AfterID: "invalid"}} {
		if _, err = (Service{}).ListIntents(ctx, f.scope, page); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid page", err)
		}
	}
	if _, err = (Service{}).ListIntents(context.Background(), f.scope, QueryPage{Limit: 10}); !errors.Is(err, ErrForbidden) {
		t.Fatal("untrusted actor", err)
	}
}
