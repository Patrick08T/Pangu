//go:build integration

package evolution

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type reservationFixture struct {
	governanceFixture
	service Service
	request CandidateRequest
	suite   *evaluation.FrozenSuite
}

func newReservationFixture(t *testing.T) reservationFixture {
	t.Helper()
	f := reservationFixture{governanceFixture: newGovernanceFixture(t)}
	connection := environmentConnection(t, f.governanceFixture)
	env, err := f.service.RegisterEnvironment(context.Background(), f.scope, Environment{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", ConnectionVersion: connection.ID, Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	node := workflow.Node{ID: "answer", Executor: "eino", Model: "offline-fixture", ConnectionVersion: connection.ID, Instruction: "回答JSON", Handoff: &workflow.HandoffContract{Objective: "回答本次输入", Inputs: "本次JSON输入", Deliverables: "JSON结果", Acceptance: "独立checker验证", FailureHandling: "报告失败，不伪造"}}
	item, err := catalog.Create(context.Background(), f.scope.Namespace, catalog.NewObject{Kind: "agent", Name: "controlled role", Spec: catalog.Spec{Agent: &node}})
	if err != nil {
		t.Fatal(err)
	}
	base, err := catalog.Publish(context.Background(), f.scope.Namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	f.suite = createEvolutionSuite(t, f.governanceFixture)
	f.request = CandidateRequest{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, PolicyRevision: 2, AgentObjectID: item.ID, BaselineVersion: base.ID, EnvironmentID: env.ID, SuiteID: f.suite.ID, Entries: []EntryTarget{{Kind: "workbench", ID: "main"}}, Budget: experience.Budget{Calls: 6, Tokens: 600}, Payload: catalog.CandidatePayload{Agent: node}}
	f.request.Payload.Agent.Instruction = "先验证JSON，再作答"
	f.policy = reservationPolicy(f)
	f.savePolicy(t)
	return f
}

func reservationPolicy(f reservationFixture) experience.Policy {
	p, _ := suitePolicyFixture()
	p.ProjectID = f.scope.ProjectID
	p.ActorID = f.scope.ActorID
	p.Revision = 2
	p.Enabled = true
	p.AutoPublish = true
	p.Roles = []string{f.request.AgentObjectID}
	p.Workflows = []string{"explicitly-unused"}
	p.Entrypoints = []string{"workbench:main"}
	p.Environments = []string{f.request.EnvironmentID}
	p.ExtractorVersion = "fixed-extractor"
	p.GeneratorVersion = "fixed-generator"
	p.LearningBudget = experience.Budget{Calls: 1, Tokens: 100}
	p.EvaluationBudget = experience.Budget{Calls: 100, Tokens: 10000}
	p.BudgetPeriodSeconds = 3600
	p.CandidateIntervalSeconds = 1
	p.Delivery = experience.DeliveryPolicy{Mode: "shadow"}
	p.Observation = experience.ObservationPolicy{WindowSeconds: 1, DeadlineSeconds: 10, MinimumSamples: 2, DegradationThreshold: .1, RetentionSeconds: 3600, SourceRetentionAuthorized: true}
	p.Evaluation.SuiteVersion = f.suite.ID
	p.Evaluation.ProtectionSet = protectionHash(f.suite)
	p.Evaluation.HoldoutSet = f.suite.HoldoutHash
	p.Evaluation.MinimumSamples = 1
	p.Evaluation.Repeats = 1
	return p
}

func createEvolutionSuite(t *testing.T, f governanceFixture) *evaluation.FrozenSuite {
	t.Helper()
	_, template := suitePolicyFixture()
	spec := template.SuiteSpec
	spec.ID = uuid.NewString()
	spec.ProjectID = f.scope.ProjectID
	spec.Rules.MinFamilies = 1
	spec.Rules.Repeats = 1
	spec.Rules.Metrics = []evaluation.MetricRule{{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1, Tolerance: 0}, {Name: "cost", Lower: 0, Upper: 100, Tolerance: 2}, {Name: "duration", Lower: 0, Upper: 100, Tolerance: 2}}
	spec.Cases = nil
	for i, partition := range []string{"holdout", "protection", "development"} {
		input := fmt.Sprintf("case %d", i)
		c, err := evaluation.CreateCase(context.Background(), f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, evaluation.CaseInput{Name: input, Input: input, Expected: `{"answer":1}`, Criteria: []string{"json_equal"}, RetainInput: true})
		if err != nil {
			t.Fatal(err)
		}
		source, err := submission.Prepare(context.Background(), f.scope.Namespace, "evolution-source", uuid.NewString(), submission.Snapshot{Purpose: "evaluation", Workflow: workflow.Definition{ProjectID: f.scope.ProjectID, Input: input}})
		if err != nil {
			t.Fatal(err)
		}
		run := uuid.NewString()
		if _, err = submission.Dispatch(context.Background(), f.scope.Namespace, source.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
			t.Fatal(err)
		}
		spec.Cases = append(spec.Cases, evaluation.SuiteCase{ID: c.ID, FamilyID: uuid.NewString(), TaskID: source.TaskID, SourceRunID: run, InputHash: hashText(input), ExpectedHash: hashText(`{"answer":1}`), CriteriaHash: hashJSON(c.Criteria), CheckerHash: CheckerHash(), Partition: partition, Critical: partition == "protection"})
	}
	suite, err := evaluation.CreateExperimentSuite(context.Background(), f.scope.Namespace, spec)
	if err != nil {
		t.Fatal(err)
	}
	return suite
}

func TestCandidateReservationClaimsTargetsAndIsIdempotent(t *testing.T) {
	f := newReservationFixture(t)
	ctx := context.Background()
	first, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != f.request.ID || first.State != "evaluating" || first.ExperimentID == "" {
		t.Fatal("未保留真实实验/占用", first)
	}
	second, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil || second.ExperimentID != first.ExperimentID {
		t.Fatal("retry重复预留", second, err)
	}
	f.request.ID = uuid.NewString()
	f.policy.Revision++
	f.request.PolicyRevision = f.policy.Revision
	f.savePolicy(t)
	if _, err = f.service.ReserveCandidate(ctx, f.scope, f.request); !errors.Is(err, ErrConflict) {
		t.Fatal("换policy绕过目标占用", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var bodies int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.catalog_experiment_candidates WHERE experiment_id=$1 AND payload IS NOT NULL`, first.ExperimentID).Scan(&bodies); err != nil || bodies != 1 {
		t.Fatal("真实候选未存储", bodies, err)
	}
}
