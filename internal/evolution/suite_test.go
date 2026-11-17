package evolution

import (
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"testing"
)

func suitePolicyFixture() (experience.Policy, *evaluation.FrozenSuite) {
	zero := 0.0
	resource := 2.0
	suite := &evaluation.FrozenSuite{SuiteSpec: evaluation.SuiteSpec{ID: "suite", ProjectID: "project", Version: 1, Rules: evaluation.DecisionRules{Primary: "acceptance", Method: "hoeffding_bonferroni", FailurePolicy: "invalidate", Confidence: .95, MinFamilies: 4, Repeats: 2, MaxCandidates: 2, MinImprovement: .1, Metrics: []evaluation.MetricRule{{Name: "acceptance", Tolerance: 0}, {Name: "cost", Tolerance: 2}, {Name: "duration", Tolerance: 2}}}, Cases: []evaluation.SuiteCase{{ID: "case", Partition: "protection", CheckerHash: CheckerHash()}}}, HoldoutHash: "holdout"}
	policy := experience.Policy{ProjectID: "project", Evaluation: experience.EvaluationPolicy{SuiteVersion: "suite", ProtectionSet: protectionHash(suite), HoldoutSet: "holdout", Grouping: "family", Repeats: 2, MinimumSamples: 4, PrimaryMetric: "acceptance", MinimumImprovement: .1, NonInferiority: &zero, UncertaintyMethod: "hoeffding_bonferroni", Confidence: .95, MaxCandidates: 2, MultipleComparison: "bonferroni", ResourceTolerance: &resource}}
	return policy, suite
}

func TestSuiteMustMatchFrozenPolicyAndIndependentChecker(t *testing.T) {
	policy, suite := suitePolicyFixture()
	if err := verifySuite(policy, suite); err != nil {
		t.Fatal(err)
	}
	changes := []func(*experience.Policy, *evaluation.FrozenSuite){
		func(p *experience.Policy, s *evaluation.FrozenSuite) { p.Evaluation.SuiteVersion = "other" },
		func(p *experience.Policy, s *evaluation.FrozenSuite) { p.Evaluation.HoldoutSet = "other" },
		func(p *experience.Policy, s *evaluation.FrozenSuite) { p.Evaluation.ProtectionSet = "other" },
		func(p *experience.Policy, s *evaluation.FrozenSuite) { s.Rules.MinImprovement = .001 },
		func(p *experience.Policy, s *evaluation.FrozenSuite) { s.Rules.MaxCandidates = 100 },
		func(p *experience.Policy, s *evaluation.FrozenSuite) { s.Rules.Repeats = 1 },
		func(p *experience.Policy, s *evaluation.FrozenSuite) { s.Cases[0].CheckerHash = "self_check" },
	}
	for _, change := range changes {
		p, s := suitePolicyFixture()
		change(&p, s)
		if verifySuite(p, s) == nil {
			t.Fatal("放宽冻结套件或伪造检查器被接受")
		}
	}
}
