//go:build integration

package approval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/google/uuid"
)

func continuationDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (f continuationMemoryFixture) policy(t *testing.T) experience.Policy {
	t.Helper()
	zero := 0.0
	policy := experience.Policy{Enabled: true, ReadEnabled: true, Roles: []string{f.role}, Environments: []string{"test"},
		ExtractorVersion: "extractor-1", GeneratorVersion: "generator-1", LearningBudget: experience.Budget{Calls: 10, Tokens: 10000},
		EvaluationBudget: experience.Budget{Calls: 10, Tokens: 10000}, BudgetPeriodSeconds: 3600, CandidateIntervalSeconds: 1,
		Evaluation: experience.EvaluationPolicy{SuiteVersion: "suite-1", ProtectionSet: "protect", HoldoutSet: "holdout", Grouping: "family", Repeats: 2,
			MinimumSamples: 2, PrimaryMetric: "acceptance", MinimumImprovement: 0.1, NonInferiority: &zero, UncertaintyMethod: "paired-bootstrap",
			Confidence: 0.95, MaxCandidates: 3, MultipleComparison: "bonferroni", ResourceTolerance: &zero},
		Delivery:    experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"submit", "plan"}, TrafficBasisPoints: 10000},
		Observation: experience.ObservationPolicy{WindowSeconds: 60, MinimumSamples: 2, DeadlineSeconds: 3600, DegradationThreshold: 0.1, RetentionSeconds: 86400, SourceRetentionAuthorized: true}}
	saved, err := learning.NewStore().SavePolicy(f.ctx, f.scope, policy, 0)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func (f continuationMemoryFixture) report(t *testing.T) experience.Source {
	t.Helper()
	run, request, node, id := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	snapshot, _ := json.Marshal(map[string]any{"workflow": map[string]any{"project_id": f.scope.ProjectID, "input": "original business"}, "created_by": f.actor.ID, "purpose": "business"})
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,created_by)
  VALUES($1::uuid,$2,$3,'submit',$1::text,$1::text,$4,$5,$6)`, request, f.actor.Namespace, f.scope.ProjectID, snapshot, run, f.actor.ID)
	f.exec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','test','source',true)`, run, f.scope.ProjectID)
	history, _ := json.Marshal(map[string]any{"id": run, "project_id": f.scope.ProjectID, "status": "succeeded", "stop_confirmed": true, "nodes": []any{map[string]any{"id": node, "node_id": "research"}}})
	f.exec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,now(),$3)`, run, f.actor.Namespace, history)
	body := []byte(`{"result":{"valid":true}}`)
	f.exec(t, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence) VALUES($1,$2,$3,'contract',$4,'{}','[]')`, id, run, node, body)
	return experience.Source{ID: id, Kind: "report", RunID: run, NodeID: "research", ProjectID: f.scope.ProjectID, Hash: continuationDigest(body)}
}

func (f *continuationMemoryFixture) activate(t *testing.T) {
	t.Helper()
	policy := f.policy(t)
	f.source = f.report(t)
	if err := learning.GrantSource(f.ctx, f.scope, f.source, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	content := experience.Content{Kind: "rule", Observation: "JSON result is valid", Practice: "Check result.valid", Conditions: "Only for JSON reports",
		Counterexamples: "Prose is not verified", Applicability: experience.Applicability{Roles: []string{f.role}, Keywords: []string{"JSON"}}}
	raw, _ := json.Marshal(content)
	checker, err := learning.RegisterChecker(f.ctx, f.scope, learning.CheckerSpec{Kind: "json_assertion_v1", ContentHash: continuationDigest(raw),
		SourceKind: "report", NodeID: f.source.NodeID, JSONPointer: "/result/valid", Expected: []byte(`true`)}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	store := learning.NewStore()
	intent, err := store.EnsureLearningIntent(f.ctx, f.scope, experience.IntentInput{SourceRunID: f.source.RunID, EvidenceRevision: 1, PolicyRevision: policy.Revision, Purpose: "business", Terminal: true, StopConfirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	f.checker, f.intent = checker.ID, intent
	version, err := store.Create(f.ctx, f.scope, experience.Version{ProjectID: f.scope.ProjectID, Revision: 1, Content: content, Sources: []experience.Source{f.source}, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := learning.CheckClaim(f.ctx, f.scope, version.ID, checker.ID, intent.ID, intent.Generation)
	if err != nil {
		t.Fatal(err)
	}
	f.version, err = store.Activate(f.ctx, f.scope, version.ID, activation)
	if err != nil {
		t.Fatal(err)
	}
	// Delivery is a persisted fixture precondition; benefit qualification is tested by learning.
	delivery, _ := json.Marshal(policy.Delivery)
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,evidence_id,actor_id) VALUES($1,2,'serving',$2,$3,$4)`, version.ID, delivery, uuid.NewString(), f.actor.ID)
}
