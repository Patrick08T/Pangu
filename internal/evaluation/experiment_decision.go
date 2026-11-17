package evaluation

import "fmt"

// EvaluateExperiment is a pure decision function, not an authorization boundary.
// The service caller must re-read live policy, dependencies and environment into
// current; never accept either receipts or current directly from an API client.
func EvaluateExperiment(s *FrozenSuite, e *FrozenExperiment, receipts []ExperimentReceipt, current ExperimentSpec) ExperimentDecision {
	if err := validateExperiment(s, e); err != nil {
		return decision("insufficient", "invalid_snapshot")
	}
	if contentHash(current) != e.Hash {
		return decision("insufficient", "drift")
	}
	index, failure := receiptIndex(s, e, receipts)
	if failure != "" {
		return decision("insufficient", failure)
	}
	if failure = checkComplete(s, index); failure != "" {
		if failure == "protection" || failure == "authorization_or_isolation" {
			return decision("rejected", failure)
		}
		return decision("insufficient", failure)
	}
	out := decision("passed", "thresholds_met")
	groups := append([]string{""}, s.Rules.Groups...)
	for _, group := range groups {
		for _, rule := range s.Rules.Metrics {
			interval := pairedInterval(s, index, rule, group)
			out.Intervals = append(out.Intervals, interval)
			if interval.Families < s.Rules.MinFamilies {
				out.Outcome, out.Category = "insufficient", "sample_count"
				continue
			}
			threshold, category := -rule.Tolerance, "non_inferiority"
			if group == "" && rule.Name == s.Rules.Primary {
				threshold, category = s.Rules.MinImprovement, "primary_threshold"
			}
			if interval.Lower < threshold && out.Outcome == "passed" {
				out.Outcome, out.Category = "rejected", category
			}
		}
	}
	return out
}

func decision(outcome, category string) ExperimentDecision {
	return ExperimentDecision{Outcome: outcome, Category: category}
}

func receiptKey(arm, caseID string, repeat int) string {
	return fmt.Sprintf("%s/%s/%d", arm, caseID, repeat)
}

func receiptIndex(s *FrozenSuite, e *FrozenExperiment, receipts []ExperimentReceipt) (map[string]ExperimentReceipt, string) {
	index := map[string]ExperimentReceipt{}
	for _, r := range receipts {
		if err := validateReceipt(s, e, r); err != nil {
			return nil, "receipt_identity"
		}
		key := receiptKey(r.Arm, r.CaseID, r.Repeat)
		if _, exists := index[key]; exists {
			return nil, "duplicate_attempt"
		}
		index[key] = r
	}
	return index, ""
}

func checkComplete(s *FrozenSuite, index map[string]ExperimentReceipt) string {
	for _, c := range s.Cases {
		if c.Partition == "development" {
			continue
		}
		for repeat := 1; repeat <= s.Rules.Repeats; repeat++ {
			for _, arm := range []string{"baseline", "candidate"} {
				r, ok := index[receiptKey(arm, c.ID, repeat)]
				if !ok {
					return "incomplete_pairs"
				}
				if !r.GatesPassed {
					return "authorization_or_isolation"
				}
				if c.Critical && !r.ProtectionPassed {
					return "protection"
				}
				if r.Status != "completed" {
					return "execution_" + r.Status
				}
				for _, m := range s.Rules.Metrics {
					value, known := r.Metrics[m.Name]
					if !known || !finite(value) {
						return "unknown_metric"
					}
					if value < m.Lower || value > m.Upper {
						return "metric_bounds"
					}
				}
			}
		}
	}
	return ""
}
