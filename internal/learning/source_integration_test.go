//go:build integration

package learning

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func (f learningFixture) report(t *testing.T) experience.Source {
	t.Helper()
	run, task, node, id := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	snapshot, _ := json.Marshal(map[string]any{"workflow": map[string]any{"project_id": f.scope.ProjectID, "input": "business input"}, "created_by": f.actor.ID, "purpose": "business"})
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,created_by)
	 VALUES($1,$2,$3,'submit',$4,$4,$5,$6,$7)`, task, f.scope.Namespace, f.scope.ProjectID, task, snapshot, run, f.actor.ID)
	f.exec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready)
	 VALUES($1,$2,'base','head','test','unused',true)`, run, f.scope.ProjectID)
	history, _ := json.Marshal(map[string]any{"id": run, "project_id": f.scope.ProjectID, "status": "succeeded", "stop_confirmed": true,
		"nodes": []any{map[string]any{"id": node, "node_id": "research"}}})
	f.exec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,now(),$3)`, run, f.scope.Namespace, history)
	body := []byte(`{"result":{"valid":true}}`)
	f.exec(t, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence)
	 VALUES($1,$2,$3,'contract',$4,'{}','[]')`, id, run, node, body)
	return experience.Source{ID: id, Kind: "report", RunID: run, NodeID: "research", ProjectID: f.scope.ProjectID, Hash: digest(body)}
}

func TestLearningRequiresConsentAndAuthenticSourceHash(t *testing.T) {
	f := newLearningFixture(t)
	f.policy(t)
	source := f.report(t)
	if _, err := ReadSource(f.ctx, f.scope, source); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("unconsented source read", err)
	}
	if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if evidence, err := ReadSource(f.ctx, f.scope, source); err != nil || digest(evidence.Body) != source.Hash {
		t.Fatal(evidence, err)
	}
	changed := source
	changed.Hash = digest([]byte("caller-forged hash"))
	if err := GrantSource(f.ctx, f.scope, changed, time.Now().Add(time.Hour)); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatal("forged source accepted", err)
	}
	if err := WithdrawSource(f.ctx, f.scope, source.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSource(f.ctx, f.scope, source); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("withdrawn source read", err)
	}
}

func TestLearningRejectsInternalAndPurgedSources(t *testing.T) {
	for _, change := range []string{"purpose", "expired", "purged"} {
		t.Run(change, func(t *testing.T) {
			f := newLearningFixture(t)
			f.policy(t)
			source := f.report(t)
			if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "purpose":
				f.exec(t, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{purpose}','"evaluation"') WHERE run_id=$1`, source.RunID)
			case "expired":
				f.exec(t, `UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1`, source.RunID)
			case "purged":
				f.exec(t, `DELETE FROM pangu.research_reports WHERE id=$1`, source.ID)
			}
			if _, err := ReadSource(f.ctx, f.scope, source); err == nil {
				t.Fatal("ineligible source read")
			}
		})
	}
}

func (f learningFixture) policy(t *testing.T) experience.Policy {
	t.Helper()
	zero := 0.0
	policy := experience.Policy{Enabled: true, ReadEnabled: true, Roles: []string{"researcher"}, Environments: []string{"test"},
		ExtractorVersion: "extractor-1", GeneratorVersion: "generator-1", LearningBudget: experience.Budget{Calls: 10, Tokens: 10000},
		EvaluationBudget: experience.Budget{Calls: 10, Tokens: 10000}, BudgetPeriodSeconds: 3600, CandidateIntervalSeconds: 1,
		Evaluation: experience.EvaluationPolicy{SuiteVersion: "suite-1", ProtectionSet: "protect", HoldoutSet: "holdout", Grouping: "family",
			Repeats: 2, MinimumSamples: 2, PrimaryMetric: "acceptance", MinimumImprovement: 0.1, NonInferiority: &zero,
			UncertaintyMethod: "paired-bootstrap", Confidence: 0.95, MaxCandidates: 3, MultipleComparison: "bonferroni", ResourceTolerance: &zero},
		Delivery: experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"business"}, TrafficBasisPoints: 10000},
		Observation: experience.ObservationPolicy{WindowSeconds: 60, MinimumSamples: 2, DeadlineSeconds: 3600, DegradationThreshold: 0.1,
			RetentionSeconds: 86400, SourceRetentionAuthorized: true}}
	out, err := f.store.SavePolicy(f.ctx, f.scope, policy, 0)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
