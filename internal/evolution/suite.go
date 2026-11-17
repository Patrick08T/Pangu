package evolution

import (
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
)

func protectionHash(suite *evaluation.FrozenSuite) string {
	cases := []evaluation.SuiteCase{}
	for _, c := range suite.Cases {
		if c.Partition == "protection" {
			cases = append(cases, c)
		}
	}
	return hashJSON(cases)
}
func verifySuite(policy experience.Policy, suite *evaluation.FrozenSuite) error {
	p, r := policy.Evaluation, suite.Rules
	if suite.ProjectID != policy.ProjectID || p.SuiteVersion != suite.ID || p.ProtectionSet != protectionHash(suite) || p.HoldoutSet != suite.HoldoutHash || p.Grouping != "family" || p.MultipleComparison != "bonferroni" {
		return ErrForbidden
	}
	if p.Repeats != r.Repeats || p.MinimumSamples != r.MinFamilies || p.PrimaryMetric != r.Primary || p.MinimumImprovement != r.MinImprovement || p.UncertaintyMethod != r.Method || p.Confidence != r.Confidence || p.MaxCandidates != r.MaxCandidates || r.FailurePolicy != "invalidate" {
		return ErrForbidden
	}
	if p.NonInferiority == nil || p.ResourceTolerance == nil {
		return ErrForbidden
	}
	for _, metric := range r.Metrics {
		limit := *p.NonInferiority
		if metric.Name == "cost" || metric.Name == "duration" {
			limit = *p.ResourceTolerance
		}
		if metric.Tolerance > limit {
			return ErrForbidden
		}
	}
	for _, c := range suite.Cases {
		if c.CheckerHash != CheckerHash() {
			return ErrChecker
		}
	}
	return nil
}
