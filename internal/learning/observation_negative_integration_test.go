//go:build integration

package learning

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func TestMemoryServingCannotCherryPickOrCountCopiedInputs(t *testing.T) {
	for _, scenario := range []string{"missing_artifact", "withdrawn_feedback", "copied_inputs"} {
		t.Run(scenario, func(t *testing.T) {
			f := newLearningFixture(t)
			version, experiment, _ := f.benefit(t)
			version = f.historicalCanary(t, version)
			var source experience.Source
			for i := 0; i < 4; i++ {
				index := i
				if scenario == "copied_inputs" {
					index = 0
				}
				source = f.historicalObservation(t, version, index)
			}
			if scenario == "missing_artifact" {
				f.historicalObservation(t, version, 4, true)
			}
			if scenario == "withdrawn_feedback" {
				if err := WithdrawSource(f.ctx, f.scope, source.ID); err != nil {
					t.Fatal(err)
				}
			}
			store := NewStoreWithBenefit(func(context.Context, pgx.Tx, experience.Scope, string) (evaluation.ExperimentSpec, error) {
				return experiment.ExperimentSpec, nil
			})
			change := experience.DeliveryChange{ExpectedRevision: version.DeliveryRevision, EvidenceID: experiment.ID, Policy: experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"business"}, TrafficBasisPoints: 10000}}
			if _, err := store.SetDelivery(f.ctx, f.scope, version.ID, change); err == nil {
				t.Fatal("incomplete/nonindependent cohort served")
			}
		})
	}
}
