package experience

import "testing"

func enabledPolicy() Policy {
	nonInferiority, resourceTolerance := .02, .1
	return Policy{Enabled: true, ReadEnabled: true, ProjectID: "project", Revision: 1,
		ActorID: "actor", Roles: []string{"role"}, ExtractorVersion: "extractor-v1",
		GeneratorVersion: "generator-v1", Environments: []string{"isolated"},
		LearningBudget: Budget{Calls: 10, Tokens: 1000}, EvaluationBudget: Budget{Calls: 20, Tokens: 2000},
		BudgetPeriodSeconds: 3600, CandidateIntervalSeconds: 60,
		Evaluation: EvaluationPolicy{SuiteVersion: "suite-v1", ProtectionSet: "guard-v1",
			HoldoutSet: "holdout-v1", Grouping: "input_family", Repeats: 2, MinimumSamples: 10,
			PrimaryMetric: "acceptance", MinimumImprovement: .01, NonInferiority: &nonInferiority,
			UncertaintyMethod: "paired", Confidence: .95, MaxCandidates: 3,
			MultipleComparison: "bonferroni", ResourceTolerance: &resourceTolerance},
		Delivery: DeliveryPolicy{Mode: "shadow"},
		Observation: ObservationPolicy{WindowSeconds: 60, MinimumSamples: 10,
			DeadlineSeconds: 600, DegradationThreshold: .05, RetentionSeconds: 86400,
			SourceRetentionAuthorized: true}}
}

func TestPolicyDefaultsRemainDisabled(t *testing.T) {
	var policy Policy
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	if policy.Enabled || policy.ReadEnabled || policy.AutoPublish {
		t.Fatal("zero policy enabled a side effect")
	}
}

func TestPolicyRequiresExplicitEnablementPrerequisites(t *testing.T) {
	cases := map[string]func(*Policy){
		"learning budget":      func(p *Policy) { p.LearningBudget.Calls = 0 },
		"evaluation budget":    func(p *Policy) { p.EvaluationBudget.Tokens = 0 },
		"roles":                func(p *Policy) { p.Roles = nil },
		"environment":          func(p *Policy) { p.Environments = nil },
		"holdout":              func(p *Policy) { p.Evaluation.HoldoutSet = "" },
		"metric":               func(p *Policy) { p.Evaluation.PrimaryMetric = "unknown" },
		"confidence":           func(p *Policy) { p.Evaluation.Confidence = 1 },
		"retention":            func(p *Policy) { p.Observation.SourceRetentionAuthorized = false },
		"canary authorization": func(p *Policy) { p.Delivery.Mode = "canary" },
		"publication scope":    func(p *Policy) { p.AutoPublish = true },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			policy := enabledPolicy()
			change(&policy)
			if policy.Validate() == nil {
				t.Fatal("incomplete enabled policy accepted")
			}
		})
	}
}

func TestPolicyAcceptsExplicitScopesAndBudgets(t *testing.T) {
	policy := enabledPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	policy.Delivery = DeliveryPolicy{Mode: "canary", TaskScope: []string{"approved"}, TrafficBasisPoints: 100}
	policy.AutoPublish = true
	policy.Workflows = []string{"workflow"}
	policy.Entrypoints = []string{"webhook:one"}
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
}
