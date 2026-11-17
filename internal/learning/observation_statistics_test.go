package learning

import (
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestObservationBoundsDoNotConfuseUnknownWithDegradation(t *testing.T) {
	policy := experience.Policy{Evaluation: experience.EvaluationPolicy{Confidence: .95, MaxCandidates: 3}, Observation: experience.ObservationPolicy{MinimumSamples: 2, DegradationThreshold: .1}}
	baseline := make([]float64, 64)
	for i := range baseline {
		baseline[i] = 1
	}
	if observationDegraded(policy, map[string]bool{}, baseline) {
		t.Fatal("unknown reported as degradation")
	}
	cohort := map[string]bool{}
	for i := 0; i < 64; i++ {
		cohort[fmt.Sprint(i)] = false
	}
	if !observationDegraded(policy, cohort, baseline) {
		t.Fatal("large verified degradation ignored")
	}
}
