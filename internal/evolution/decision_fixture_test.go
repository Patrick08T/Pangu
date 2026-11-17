//go:build integration

package evolution

import (
	"context"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func newDecisionFixture(t *testing.T) reservationFixture {
	t.Helper()
	f := newReservationFixture(t)
	spec := f.suite.SuiteSpec
	spec.ID = uuid.NewString()
	spec.Cases = nil
	for i := 0; i < 14; i++ {
		c := decisionSourceCase(t, f.governanceFixture, i)
		if i == 12 {
			c.Partition = "protection"
			c.Critical = true
		}
		if i == 13 {
			c.Partition = "development"
		}
		spec.Cases = append(spec.Cases, c)
	}
	spec.Rules.Confidence = .51
	spec.Rules.MaxCandidates = 1
	spec.Rules.MinFamilies = 12
	spec.Rules.MinImprovement = .1
	spec.Rules.Metrics = []evaluation.MetricRule{{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1, Tolerance: 0}, {Name: "cost", Lower: 0, Upper: 1, Tolerance: 1}, {Name: "duration", Lower: 0, Upper: 1, Tolerance: 1}}
	var err error
	f.suite, err = evaluation.CreateExperimentSuite(context.Background(), f.scope.Namespace, spec)
	if err != nil {
		t.Fatal(err)
	}
	f.request.SuiteID = f.suite.ID
	f.request.PolicyRevision = 3
	f.request.Budget = experience.Budget{Calls: 26, Tokens: 2600}
	f.policy.Revision = 3
	f.policy.Evaluation.SuiteVersion = f.suite.ID
	f.policy.Evaluation.ProtectionSet = protectionHash(f.suite)
	f.policy.Evaluation.HoldoutSet = f.suite.HoldoutHash
	f.policy.Evaluation.MinimumSamples = 12
	f.policy.Evaluation.Confidence = .51
	f.policy.Evaluation.MaxCandidates = 1
	f.policy.Evaluation.MinimumImprovement = .1
	tolerance := 1.
	f.policy.Evaluation.ResourceTolerance = &tolerance
	f.savePolicy(t)
	return f
}

func decisionSourceCase(t *testing.T, f governanceFixture, i int) evaluation.SuiteCase {
	t.Helper()
	ctx := context.Background()
	input := fmt.Sprintf("independent decision case %d", i)
	c, err := evaluation.CreateCase(ctx, f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, evaluation.CaseInput{Name: input, Input: input, Expected: `{"answer":1}`, Criteria: []string{"json_equal"}, RetainInput: true})
	if err != nil {
		t.Fatal(err)
	}
	source, err := submission.Prepare(ctx, f.scope.Namespace, "evolution-source", uuid.NewString(), submission.Snapshot{Purpose: "evaluation", Workflow: workflow.Definition{ProjectID: f.scope.ProjectID, Input: input}})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, f.scope.Namespace, source.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	return evaluation.SuiteCase{ID: c.ID, FamilyID: uuid.NewString(), TaskID: source.TaskID, SourceRunID: run, InputHash: hashText(input), ExpectedHash: hashText(`{"answer":1}`), CriteriaHash: hashJSON(c.Criteria), CheckerHash: CheckerHash(), Partition: "holdout"}
}

func completedDecisionFixture(t *testing.T) (reservationFixture, *Intent) {
	t.Helper()
	return completeDecisionFixture(t, newDecisionFixture(t))
}

func completeDecisionFixture(t *testing.T, f reservationFixture) (reservationFixture, *Intent) {
	t.Helper()
	evidence := &executionEvidence{}
	f.service.Evidence = evidence
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range f.suite.Cases {
		if c.Partition == "development" {
			continue
		}
		for _, arm := range []string{"baseline", "candidate"} {
			plan, err := f.service.PrepareCase(ctx, f.scope, CaseKey{IntentID: intent.ID, Generation: intent.Generation, Arm: arm, CaseID: c.ID, Repeat: 1})
			if err != nil {
				t.Fatal(err)
			}
			evidence.result = actualMachineFixture(plan)
			if arm == "baseline" && c.Partition == "holdout" {
				evidence.result.Text = `{"answer":0}`
			}
			if err = f.service.RecordCaseResult(ctx, f.scope, plan.Key, plan.RequestID); err != nil {
				t.Fatal(err)
			}
		}
	}
	return f, intent
}
