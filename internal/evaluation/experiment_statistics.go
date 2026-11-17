package evaluation

import (
	"math"
	"sort"
)

type familyMean struct {
	Mean  float64
	Count int
}

// pairedInterval uses Hoeffding's inequality on independent family means.
// Replicates and multiple cases within one family are averaged, never counted
// as independent samples. Observations in [L,U] give differences in [L-U,U-L].
// Two-sided alpha is Bonferroni-allocated across every preregistered candidate,
// metric and group; there is no optional stopping or result-driven retry.
func pairedInterval(s *FrozenSuite, index map[string]ExperimentReceipt, rule MetricRule, group string) MetricInterval {
	families := map[string]familyMean{}
	for _, c := range s.Cases {
		if c.Partition != "holdout" || group != "" && c.Group != group {
			continue
		}
		for repeat := 1; repeat <= s.Rules.Repeats; repeat++ {
			baseline := index[receiptKey("baseline", c.ID, repeat)].Metrics[rule.Name]
			candidate := index[receiptKey("candidate", c.ID, repeat)].Metrics[rule.Name]
			difference := candidate - baseline
			if !rule.HigherBetter {
				difference = -difference
			}
			f := families[c.FamilyID]
			f.Count++
			f.Mean += (difference - f.Mean) / float64(f.Count)
			families[c.FamilyID] = f
		}
	}
	keys := make([]string, 0, len(families))
	for key := range families {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	mean := 0.
	for i, key := range keys {
		mean += (families[key].Mean - mean) / float64(i+1)
	}
	out := MetricInterval{Metric: rule.Name, Group: group, Families: len(keys), Mean: mean}
	if len(keys) == 0 {
		return out
	}
	comparisons := float64(s.Rules.MaxCandidates) * float64(len(s.Rules.Metrics)) * float64(1+len(s.Rules.Groups))
	// Log-domain allocation avoids underflow for very small registered alpha.
	logTerm := math.Log(2) + math.Log(comparisons) - math.Log1p(-s.Rules.Confidence)
	rangeWidth := rule.Upper - rule.Lower
	radius := 2 * rangeWidth * math.Sqrt(logTerm/(2*float64(len(keys))))
	out.Lower = math.Max(-rangeWidth, mean-radius)
	out.Upper = math.Min(rangeWidth, mean+radius)
	return out
}
