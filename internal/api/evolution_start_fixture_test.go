//go:build integration

package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

type evolutionStartAPIFixture struct {
	experienceFixture
	backend    scheduler.Backend
	input      evolutionCandidateInput
	calls      *atomic.Int32
	suite      *evaluation.FrozenSuite
	flowObject string
}

func newEvolutionStartAPI(t *testing.T) evolutionStartAPIFixture {
	t.Helper()
	f := evolutionStartAPIFixture{experienceFixture: experienceAPI(t), calls: &atomic.Int32{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	claims, _ := json.Marshal(map[string]any{"server_url": server.URL, "grpc_broadcast_address": "127.0.0.1:1", "exp": time.Now().Add(time.Hour).Unix(), "sub": uuid.NewString()})
	token := "test." + base64.RawURLEncoding.EncodeToString(claims) + ".fixture"
	client, err := hatchet.NewClient(hatchet.WithToken(token), hatchet.WithNamespace("api-evolution-start-"+uuid.NewString()), hatchet.WithHostPort("127.0.0.1", 1), hatchet.WithTLSConfig(nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	f.backend = scheduler.NewBackend(client)
	capabilityExec(t, `UPDATE pangu.projects SET namespace=$1 WHERE namespace=$2`, f.backend.Namespace(), f.namespace)
	capabilityExec(t, `UPDATE pangu.users SET namespace=$1 WHERE namespace=$2`, f.backend.Namespace(), f.namespace)
	f.namespace = f.backend.Namespace()
	f.handler = New(f.backend, identity.Service{Namespace: f.namespace})
	f.configure(t, server.URL)
	f.suite = f.createSuite(t)
	f.configurePolicy(t)
	return f
}

func (f evolutionStartAPIFixture) publish(t *testing.T, kind, name string, spec catalog.Spec) *catalog.Version {
	t.Helper()
	item, err := catalog.Create(context.Background(), f.namespace, catalog.NewObject{Kind: kind, Name: name, Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(context.Background(), f.namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func (f *evolutionStartAPIFixture) configure(t *testing.T, url string) {
	t.Helper()
	t.Setenv("PANGU_SECRET_EVOLUTION_START_TEST", "test-not-a-secret")
	connection := f.publish(t, "connection", "model", catalog.Spec{Connection: &catalog.Connection{Provider: "openai", Model: "fixture", BaseURL: url, SecretRef: "PANGU_SECRET_EVOLUTION_START_TEST", Price: &workflow.Price{Currency: "USD", InputPerMillionMicros: 1000000, OutputPerMillionMicros: 2000000}}})
	scope := evolution.Scope{Namespace: f.namespace, ProjectID: f.project, ActorID: f.actor}
	env, err := f.backend.EvolutionService().RegisterEnvironment(context.Background(), scope, evolution.Environment{ID: uuid.NewString(), ProjectID: f.project, Backend: "eino_json_v1", ConnectionVersion: connection.ID, Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	node := workflow.Node{ID: "role", Executor: "eino", Model: "fixture", ConnectionVersion: connection.ID, Instruction: "baseline-private-instruction", Handoff: &workflow.HandoffContract{Objective: "answer", Inputs: "case", Deliverables: "JSON", Acceptance: "independent check", FailureHandling: "report error"}}
	role := f.publish(t, "agent", "role", catalog.Spec{Agent: &node})
	flow := f.publish(t, "workflow", "full flow", catalog.Spec{Flow: &catalog.Flow{Nodes: []catalog.Binding{{ID: "draft", AgentVersion: role.ID, Task: "draft-task"}, {ID: "answer", AgentVersion: role.ID, Task: "answer-task", DependsOn: []string{"draft"}}}}})
	f.flowObject = flow.ObjectID
	f.input = evolutionCandidateInput{ID: uuid.NewString(), PolicyRevision: 1, BaselineVersion: role.ID, EnvironmentID: env.ID, FlowVersion: flow.ID, BindingIDs: []string{"draft", "answer"}, Entries: []evolution.EntryTarget{{Kind: "workbench", ID: "main"}}, Budget: experience.Budget{Calls: 12, Tokens: 4000}, Instruction: "candidate-private-instruction"}
}

func (f *evolutionStartAPIFixture) configurePolicy(t *testing.T) {
	t.Helper()
	zero, resource := 0.0, 2.0
	protection := []evaluation.SuiteCase{}
	for _, c := range f.suite.Cases {
		if c.Partition == "protection" {
			protection = append(protection, c)
		}
	}
	version, err := catalog.ReadVersionScoped(context.Background(), f.namespace, f.input.BaselineVersion, catalog.VersionScope{ProjectID: f.project})
	if err != nil {
		t.Fatal(err)
	}
	f.input.SuiteID = f.suite.ID
	p := experience.Policy{ProjectID: f.project, ActorID: f.actor, Revision: 1, Enabled: true, AutoPublish: true, Roles: []string{version.ObjectID}, Workflows: []string{f.flowObject}, Entrypoints: []string{"workbench:main"}, Environments: []string{f.input.EnvironmentID}, ExtractorVersion: "fixed-extractor", GeneratorVersion: "fixed-generator", LearningBudget: experience.Budget{Calls: 1, Tokens: 100}, EvaluationBudget: experience.Budget{Calls: 100, Tokens: 10000}, BudgetPeriodSeconds: 3600, CandidateIntervalSeconds: 1, Delivery: experience.DeliveryPolicy{Mode: "shadow"}, Observation: experience.ObservationPolicy{WindowSeconds: 1, DeadlineSeconds: 10, MinimumSamples: 2, DegradationThreshold: .1, RetentionSeconds: 3600, SourceRetentionAuthorized: true}}
	p.Evaluation = experience.EvaluationPolicy{SuiteVersion: f.suite.ID, ProtectionSet: evolutionStartHash(protection), HoldoutSet: f.suite.HoldoutHash, Grouping: "family", Repeats: 1, MinimumSamples: 1, PrimaryMetric: "acceptance", MinimumImprovement: .1, NonInferiority: &zero, ResourceTolerance: &resource, UncertaintyMethod: "hoeffding_bonferroni", Confidence: .95, MaxCandidates: 2, MultipleComparison: "bonferroni"}
	raw, _ := json.Marshal(p)
	capabilityExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,1,$3,$4)`, f.namespace, f.project, f.actor, raw)
}
