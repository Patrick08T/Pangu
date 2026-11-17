package learning

import (
	"context"
	"slices"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func verifyBenefitWithPlan(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, change experience.DeliveryChange, reader BenefitPlanReader) error {
	if !validID(change.EvidenceID) {
		return experience.ErrForbidden
	}
	policy, err := policyTx(ctx, tx, scope)
	if err != nil {
		return err
	}
	current, err := reader(ctx, tx, scope, change.EvidenceID)
	if err != nil {
		return err
	}
	experiment, err := evaluation.ReadExperimentTx(ctx, tx, scope.Namespace, scope.ProjectID, change.EvidenceID)
	if err != nil {
		return err
	}
	if err = matchBenefit(policy, version, experiment.ExperimentSpec); err != nil {
		return err
	}
	suite, err := evaluation.ReadExperimentSuiteTx(ctx, tx, scope.Namespace, experiment.SuiteID)
	if err != nil {
		return err
	}
	if err = matchBenefitSuite(policy, suite); err != nil {
		return err
	}
	decision, err := evaluation.EvaluateStoredExperimentTx(ctx, tx, scope.Namespace, scope.ProjectID, experiment.ID, current)
	if err != nil {
		return err
	}
	if decision.Outcome != "passed" {
		return experience.ErrForbidden
	}
	if change.Policy.Mode == "serving" {
		return verifyMemoryObservations(ctx, tx, scope, version, policy, suite, experiment)
	}
	return nil
}

func matchBenefit(policy experience.Policy, version experience.Version, spec evaluation.ExperimentSpec) error {
	hash, err := MemoryCombinationHash([]experience.Reference{{VersionID: version.ID, BodyHash: version.BodyHash}})
	if err != nil {
		return err
	}
	if spec.Kind != "memory_benefit" || spec.Purpose != "evaluation" || spec.LogicalProjectID != version.ProjectID ||
		int64(spec.PolicyRevision) != policy.Revision || !slices.Contains(policy.Environments, spec.EnvironmentID) ||
		spec.Baseline.RoleHash != spec.Candidate.RoleHash || spec.Baseline.MemoryHash != evaluation.EmptyMemoryHash || spec.Candidate.MemoryHash != hash {
		return experience.ErrForbidden
	}
	if int64(spec.Budget.MaxCalls) > policy.EvaluationBudget.Calls || spec.Budget.MaxTokens > policy.EvaluationBudget.Tokens {
		return experience.ErrForbidden
	}
	return nil
}

func matchBenefitSuite(policy experience.Policy, suite *evaluation.FrozenSuite) error {
	p, r := policy.Evaluation, suite.Rules
	protection := []evaluation.SuiteCase{}
	for _, c := range suite.Cases {
		if c.Partition == "protection" {
			protection = append(protection, c)
		}
	}
	if p.PrimaryMetric != "acceptance" || suite.ProjectID != policy.ProjectID || p.SuiteVersion != suite.ID ||
		p.ProtectionSet != jsonHash(protection) || p.HoldoutSet != suite.HoldoutHash || p.Grouping != "family" || p.MultipleComparison != "bonferroni" {
		return experience.ErrForbidden
	}
	if p.Repeats != r.Repeats || p.MinimumSamples != r.MinFamilies || p.PrimaryMetric != r.Primary || p.MinimumImprovement != r.MinImprovement ||
		p.UncertaintyMethod != r.Method || p.Confidence != r.Confidence || p.MaxCandidates != r.MaxCandidates || r.FailurePolicy != "invalidate" {
		return experience.ErrForbidden
	}
	if p.NonInferiority == nil || p.ResourceTolerance == nil {
		return experience.ErrForbidden
	}
	for _, metric := range r.Metrics {
		limit := *p.NonInferiority
		if metric.Name == "cost" || metric.Name == "duration" {
			limit = *p.ResourceTolerance
		}
		if metric.Tolerance > limit {
			return experience.ErrForbidden
		}
	}
	return nil
}
