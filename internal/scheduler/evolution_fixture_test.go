//go:build integration

package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type evolutionFixture struct {
	scope   evolution.Scope
	service evolution.Service
	request evolution.CandidateRequest
	suite   *evaluation.FrozenSuite
}

func (f evolutionFixture) caseID() string {
	for _, c := range f.suite.Cases {
		if c.Partition == "holdout" {
			return c.ID
		}
	}
	panic("fixture has no holdout case")
}

func testEvolutionHash(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func testEvolutionText(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func newEvolutionFixture(t *testing.T, url string) evolutionFixture {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Fatal("requires independent test database")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_SECRET_EVOLUTION_DRIVER_TEST", "test-not-a-secret")
	ctx := context.Background()
	if err := database.Check(ctx); err != nil {
		t.Fatal(err)
	}
	f := evolutionFixture{scope: evolution.Scope{Namespace: "evo-driver-" + uuid.NewString(), ProjectID: uuid.NewString(), ActorID: uuid.NewString()}, service: evolution.Service{Evidence: EvolutionEvidence{}}}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'evolution driver','unused','main','test','git')`, f.scope.ProjectID, f.scope.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1,$2,'test','test',$3,true)`, f.scope.ActorID, f.scope.Namespace, f.scope.ActorID)
	if err != nil {
		t.Fatal(err)
	}
	f.configure(t, url)
	f.suite = f.createSuite(t)
	f.configurePolicy(t)
	return f
}

func (f *evolutionFixture) publish(t *testing.T, kind, name string, spec catalog.Spec) *catalog.Version {
	t.Helper()
	ctx := context.Background()
	item, err := catalog.Create(ctx, f.scope.Namespace, catalog.NewObject{Kind: kind, Name: name, Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, f.scope.Namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func (f *evolutionFixture) configure(t *testing.T, url string) {
	t.Helper()
	ctx := context.Background()
	connection := f.publish(t, "connection", "model", catalog.Spec{Connection: &catalog.Connection{Provider: "openai", Model: "fixture", BaseURL: url, SecretRef: "PANGU_SECRET_EVOLUTION_DRIVER_TEST", Price: &workflow.Price{Currency: "USD", InputPerMillionMicros: 1000000, OutputPerMillionMicros: 2000000}}})
	env, err := f.service.RegisterEnvironment(ctx, f.scope, evolution.Environment{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", ConnectionVersion: connection.ID, Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	node := workflow.Node{ID: "role", Executor: "eino", Model: "fixture", ConnectionVersion: connection.ID, Instruction: "Produce JSON", Handoff: &workflow.HandoffContract{Objective: "answer", Inputs: "case", Deliverables: "JSON", Acceptance: "independent check", FailureHandling: "report error"}}
	role := f.publish(t, "agent", "role", catalog.Spec{Agent: &node})
	flow := f.publish(t, "workflow", "full flow", catalog.Spec{Flow: &catalog.Flow{Nodes: []catalog.Binding{{ID: "draft", AgentVersion: role.ID, Task: "draft-task"}, {ID: "answer", AgentVersion: role.ID, Task: "answer-task", DependsOn: []string{"draft"}}}}})
	f.request = evolution.CandidateRequest{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, PolicyRevision: 1, AgentObjectID: role.ObjectID, FlowObjectID: flow.ObjectID, BaselineVersion: role.ID, EnvironmentID: env.ID, Entries: []evolution.EntryTarget{{Kind: "workbench", ID: "main"}}, Budget: experience.Budget{Calls: 12, Tokens: 4000}, Payload: catalog.CandidatePayload{Agent: node, FlowVersion: flow.ID, BindingIDs: []string{"draft", "answer"}}}
	f.request.Payload.Agent.Instruction = "Check your JSON before answering"
}

func (f *evolutionFixture) createSuite(t *testing.T) *evaluation.FrozenSuite {
	t.Helper()
	ctx := context.Background()
	spec := evaluation.SuiteSpec{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Version: 1, Rules: evaluation.DecisionRules{Primary: "acceptance", Method: "hoeffding_bonferroni", FailurePolicy: "invalidate", Confidence: .95, MinFamilies: 1, Repeats: 1, MaxCandidates: 2, MinImprovement: .1, Metrics: []evaluation.MetricRule{{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1}, {Name: "cost", Lower: 0, Upper: 100, Tolerance: 2}, {Name: "duration", Lower: 0, Upper: 100, Tolerance: 2}}}}
	for i, partition := range []string{"holdout", "protection", "development"} {
		input := fmt.Sprintf("private case %d", i)
		expected := `{"answer":"checker-only-value"}`
		c, err := evaluation.CreateCase(ctx, f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, evaluation.CaseInput{Name: "case", Input: input, Expected: expected, Criteria: []string{"json_equal"}, RetainInput: true})
		if err != nil {
			t.Fatal(err)
		}
		s, err := submission.Prepare(ctx, f.scope.Namespace, "source", uuid.NewString(), submission.Snapshot{Purpose: "evaluation", Workflow: workflow.Definition{ProjectID: f.scope.ProjectID, Input: input}})
		if err != nil {
			t.Fatal(err)
		}
		run := uuid.NewString()
		if _, err = submission.Dispatch(ctx, f.scope.Namespace, s.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
			t.Fatal(err)
		}
		spec.Cases = append(spec.Cases, evaluation.SuiteCase{ID: c.ID, FamilyID: uuid.NewString(), TaskID: s.TaskID, SourceRunID: run, InputHash: testEvolutionText(input), ExpectedHash: testEvolutionText(expected), CriteriaHash: testEvolutionHash(c.Criteria), CheckerHash: evolution.CheckerHash(), Partition: partition, Critical: partition == "protection"})
	}
	suite, err := evaluation.CreateExperimentSuite(ctx, f.scope.Namespace, spec)
	if err != nil {
		t.Fatal(err)
	}
	return suite
}

func (f *evolutionFixture) configurePolicy(t *testing.T) {
	t.Helper()
	zero, resource := 0.0, 2.0
	protection := []evaluation.SuiteCase{}
	for _, c := range f.suite.Cases {
		if c.Partition == "protection" {
			protection = append(protection, c)
		}
	}
	f.request.SuiteID = f.suite.ID
	p := experience.Policy{ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID, Revision: 1, Enabled: true, AutoPublish: true, Roles: []string{f.request.AgentObjectID}, Workflows: []string{f.request.FlowObjectID}, Entrypoints: []string{"workbench:main"}, Environments: []string{f.request.EnvironmentID}, ExtractorVersion: "fixed-extractor", GeneratorVersion: "fixed-generator", LearningBudget: experience.Budget{Calls: 1, Tokens: 100}, EvaluationBudget: experience.Budget{Calls: 100, Tokens: 10000}, BudgetPeriodSeconds: 3600, CandidateIntervalSeconds: 1, Delivery: experience.DeliveryPolicy{Mode: "shadow"}, Observation: experience.ObservationPolicy{WindowSeconds: 1, DeadlineSeconds: 10, MinimumSamples: 2, DegradationThreshold: .1, RetentionSeconds: 3600, SourceRetentionAuthorized: true}}
	p.Evaluation = experience.EvaluationPolicy{SuiteVersion: f.suite.ID, ProtectionSet: testEvolutionHash(protection), HoldoutSet: f.suite.HoldoutHash, Grouping: "family", Repeats: 1, MinimumSamples: 1, PrimaryMetric: "acceptance", MinimumImprovement: .1, NonInferiority: &zero, ResourceTolerance: &resource, UncertaintyMethod: "hoeffding_bonferroni", Confidence: .95, MaxCandidates: 2, MultipleComparison: "bonferroni"}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	data, _ := json.Marshal(p)
	_, err = conn.Exec(context.Background(), `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,1,$3,$4)`, f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, data)
	if err != nil {
		t.Fatal(err)
	}
}
