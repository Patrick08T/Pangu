//go:build integration

package learning

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func TestLearningBenefitUsesRealReceiptsAndTrustedCurrentPlan(t *testing.T) {
	f := newLearningFixture(t)
	version, experiment, _ := f.benefit(t)
	current := experiment.ExperimentSpec
	reader := func(ctx context.Context, tx pgx.Tx, scope experience.Scope, id string) (evaluation.ExperimentSpec, error) {
		if scope.ProjectID != f.scope.ProjectID || id != experiment.ID {
			return evaluation.ExperimentSpec{}, experience.ErrForbidden
		}
		return current, nil
	}
	change := experience.DeliveryChange{ExpectedRevision: version.DeliveryRevision, EvidenceID: experiment.ID,
		Policy: experience.DeliveryPolicy{Mode: "canary", TaskScope: []string{"business"}, TrafficBasisPoints: 1000}}
	if _, err := f.store.SetDelivery(f.ctx, f.scope, version.ID, change); err == nil {
		t.Fatal("missing trusted plan reader accepted")
	}
	current.ConfigHash = digest([]byte("changed current configuration"))
	store := NewStoreWithBenefit(reader)
	if _, err := store.SetDelivery(f.ctx, f.scope, version.ID, change); err == nil {
		t.Fatal("current environment drift accepted")
	}
	current = experiment.ExperimentSpec
	canary, err := store.SetDelivery(f.ctx, f.scope, version.ID, change)
	if err != nil || canary.Delivery != "canary" {
		t.Fatal("real passing paired evidence rejected", err)
	}
	change.ExpectedRevision, change.Policy.Mode = canary.DeliveryRevision, "serving"
	if _, err = store.SetDelivery(f.ctx, f.scope, version.ID, change); err == nil {
		t.Fatal("offline benefit replaced real canary observation")
	}
	f.exec(t, `UPDATE pangu.evaluation_cases SET archived=true WHERE id=(SELECT case_id FROM pangu.evaluation_experiment_receipts WHERE experiment_id=$1 LIMIT 1)`, experiment.ID)
	change.Policy.Mode = "canary"
	if _, err = store.SetDelivery(f.ctx, f.scope, version.ID, change); err == nil {
		t.Fatal("revoked experiment source accepted")
	}
}
