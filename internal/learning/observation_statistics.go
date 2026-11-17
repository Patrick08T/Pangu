package learning

import (
	"context"
	"encoding/json"
	"math"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func observationBaseline(ctx context.Context, tx pgx.Tx, suite *evaluation.FrozenSuite, experiment *evaluation.FrozenExperiment) ([]float64, error) {
	rows, err := tx.Query(ctx, `SELECT receipt FROM pangu.evaluation_experiment_receipts WHERE experiment_id=$1 AND generation=$2 AND arm='baseline'`, experiment.ID, experiment.Generation)
	if err != nil {
		return nil, err
	}
	blobs, err := pgx.CollectRows(rows, pgx.RowTo[[]byte])
	if err != nil {
		return nil, err
	}
	cases := map[string]string{}
	for _, c := range suite.Cases {
		if c.Partition == "holdout" {
			cases[c.ID] = c.FamilyID
		}
	}
	type aggregate struct {
		sum   float64
		count int
	}
	families := map[string]aggregate{}
	for _, data := range blobs {
		var receipt evaluation.ExperimentReceipt
		if err = json.Unmarshal(data, &receipt); err != nil {
			return nil, err
		}
		family, ok := cases[receipt.CaseID]
		if !ok {
			continue
		}
		value, ok := receipt.Metrics["acceptance"]
		if !ok || math.IsNaN(value) || value < 0 || value > 1 {
			return nil, experience.ErrInvalidated
		}
		group := families[family]
		group.sum += value
		group.count++
		families[family] = group
	}
	values := []float64{}
	for _, group := range families {
		values = append(values, group.sum/float64(group.count))
	}
	if len(values) < suite.Rules.MinFamilies {
		return nil, experience.ErrForbidden
	}
	return values, nil
}

func observationBounds(policy experience.Policy, cohort map[string]bool, baseline []float64) (float64, float64, float64, float64, bool) {
	if len(cohort) < policy.Observation.MinimumSamples || len(baseline) == 0 {
		return 0, 0, 0, 0, false
	}
	accepted := 0.
	for _, value := range cohort {
		if value {
			accepted++
		}
	}
	mean := 0.
	for _, value := range baseline {
		mean += value
	}
	mean /= float64(len(baseline))
	alpha := (1 - policy.Evaluation.Confidence) / (2 * float64(policy.Evaluation.MaxCandidates))
	if alpha <= 0 || alpha >= 1 {
		return 0, 0, 0, 0, false
	}
	observed := accepted / float64(len(cohort))
	radius := math.Sqrt(math.Log(1/alpha) / (2 * float64(len(cohort))))
	baseRadius := math.Sqrt(math.Log(1/alpha) / (2 * float64(len(baseline))))
	return math.Max(0, observed-radius), math.Min(1, observed+radius), math.Max(0, mean-baseRadius), math.Min(1, mean+baseRadius), true
}

func observationNonInferior(policy experience.Policy, cohort map[string]bool, baseline []float64) bool {
	lower, _, _, baseUpper, ok := observationBounds(policy, cohort, baseline)
	return ok && lower >= baseUpper-policy.Observation.DegradationThreshold
}

func observationDegraded(policy experience.Policy, cohort map[string]bool, baseline []float64) bool {
	_, upper, baseLower, _, ok := observationBounds(policy, cohort, baseline)
	return ok && upper < baseLower-policy.Observation.DegradationThreshold
}
