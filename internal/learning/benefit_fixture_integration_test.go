//go:build integration

package learning

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (f learningFixture) benefitSuite(t *testing.T) *evaluation.FrozenSuite {
	t.Helper()
	spec := evaluation.SuiteSpec{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Version: 1,
		Rules: evaluation.DecisionRules{Method: "hoeffding_bonferroni", FailurePolicy: "invalidate", Confidence: .95,
			MinFamilies: 64, Repeats: 2, MaxCandidates: 3, Primary: "acceptance", MinImprovement: .1,
			Metrics: []evaluation.MetricRule{{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1, Tolerance: 0},
				{Name: "cost", Lower: 0, Upper: 1, Tolerance: 1}, {Name: "duration", Lower: 0, Upper: 1, Tolerance: 1}}}}
	tx := f.transaction(t)
	for i := 0; i < 66; i++ {
		spec.Cases = append(spec.Cases, f.benefitCase(t, tx, i))
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	suite, err := evaluation.CreateExperimentSuite(f.ctx, f.scope.Namespace, spec)
	if err != nil {
		t.Fatal(err)
	}
	return suite
}

func (f learningFixture) benefitCase(t *testing.T, tx pgx.Tx, i int) evaluation.SuiteCase {
	t.Helper()
	id, request, run := uuid.NewString(), uuid.NewString(), uuid.NewString()
	input := fmt.Sprintf("independent hidden input %d", i)
	criteria := []string{"independent checker"}
	snapshot, _ := json.Marshal(map[string]any{"workflow": map[string]string{"project_id": f.scope.ProjectID, "input": input}})
	_, err := tx.Exec(f.ctx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,created_by,input_hash)
 VALUES($1,$2,$3,'submit',$4,$4,$5,$6,$7,$8)`, request, f.scope.Namespace, f.scope.ProjectID, request, snapshot, run, f.actor.ID, digest([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(f.ctx, `INSERT INTO pangu.evaluation_cases(id,namespace,project_id,actor_id,name,input,input_hash,expected,criteria)
 VALUES($1,$2,$3,$4,$5,$5,$6,'expected',$7)`, id, f.scope.Namespace, f.scope.ProjectID, f.actor.ID, input, digest([]byte(input)), criteria)
	if err != nil {
		t.Fatal(err)
	}
	partition := "holdout"
	if i == 64 {
		partition = "protection"
	}
	if i == 65 {
		partition = "development"
	}
	return evaluation.SuiteCase{ID: id, FamilyID: uuid.NewString(), TaskID: request, SourceRunID: run, InputHash: digest([]byte(input)),
		ExpectedHash: digest([]byte("expected")), CriteriaHash: jsonHash(criteria), CheckerHash: digest([]byte("checker")), Partition: partition, Critical: i == 64}
}

func (f learningFixture) benefit(t *testing.T) (experience.Version, *evaluation.FrozenExperiment, *evaluation.FrozenSuite) {
	t.Helper()
	suite := f.benefitSuite(t)
	policy := f.policy(t)
	policy.Evaluation.SuiteVersion = suite.ID
	protection := []evaluation.SuiteCase{}
	for _, c := range suite.Cases {
		if c.Partition == "protection" {
			protection = append(protection, c)
		}
	}
	policy.Evaluation.ProtectionSet, policy.Evaluation.HoldoutSet = jsonHash(protection), suite.HoldoutHash
	policy.Evaluation.UncertaintyMethod = "hoeffding_bonferroni"
	policy.Evaluation.MinimumSamples = 64
	tolerance := 1.
	policy.Evaluation.ResourceTolerance = &tolerance
	policy.EvaluationBudget = experience.Budget{Calls: 10000, Tokens: 1000000}
	environment := uuid.NewString()
	policy.Environments = []string{environment}
	var err error
	policy, err = f.store.SavePolicy(f.ctx, f.scope, policy, policy.Revision)
	if err != nil {
		t.Fatal(err)
	}
	version, intent, checker := f.claim(t, policy)
	activation, err := CheckClaim(f.ctx, f.scope, version.ID, checker.ID, intent.ID, intent.Generation)
	if err != nil {
		t.Fatal(err)
	}
	version, err = f.store.Activate(f.ctx, f.scope, version.ID, activation)
	if err != nil {
		t.Fatal(err)
	}
	memory, _ := MemoryCombinationHash([]experience.Reference{{VersionID: version.ID, BodyHash: version.BodyHash}})
	hash := digest([]byte("trusted environment"))
	spec := evaluation.ExperimentSpec{ID: uuid.NewString(), Kind: "memory_benefit", Purpose: "evaluation", LogicalProjectID: f.scope.ProjectID,
		EnvironmentID: environment, EnvironmentHash: hash, SuiteID: suite.ID, SuiteHash: suite.Hash, PolicyRevision: int(policy.Revision), Generation: 1,
		BaselineVersion: uuid.NewString(), CandidateHash: memory, SourceBaseline: hash, ConfigHash: hash,
		Budget:    evaluation.ExperimentBudget{MaxCalls: 10000, MaxTokens: 1000000},
		Baseline:  evaluation.ExperimentArm{RoleHash: hash, CompiledHash: hash, MemoryHash: evaluation.EmptyMemoryHash},
		Candidate: evaluation.ExperimentArm{RoleHash: hash, CompiledHash: hash, MemoryHash: memory}}
	experiment, err := evaluation.ReserveExperiment(f.ctx, f.scope.Namespace, spec)
	if err != nil {
		t.Fatal(err)
	}
	f.benefitReceipts(t, suite, experiment)
	return version, experiment, suite
}

func (f learningFixture) benefitReceipts(t *testing.T, s *evaluation.FrozenSuite, e *evaluation.FrozenExperiment) {
	t.Helper()
	tx := f.transaction(t)
	for _, c := range s.Cases {
		if c.Partition == "development" {
			continue
		}
		for _, arm := range []string{"baseline", "candidate"} {
			for repeat := 1; repeat <= s.Rules.Repeats; repeat++ {
				hash, err := evaluation.ExecutionInputHash(s, e, arm, c.ID, repeat)
				if err != nil {
					t.Fatal(err)
				}
				acceptance := 0.
				compiled := e.Baseline.CompiledHash
				if arm == "candidate" {
					acceptance = 1
					compiled = e.Candidate.CompiledHash
				}
				receipt := evaluation.ExperimentReceipt{ExperimentID: e.ID, ExperimentHash: e.Hash, Generation: e.Generation, Arm: arm, CaseID: c.ID,
					Repeat: repeat, Attempt: 1, ExecutionInputHash: hash, EnvironmentHash: e.EnvironmentHash, CompiledHash: compiled, CheckerHash: c.CheckerHash,
					EvidenceHash: digest([]byte("actual independent receipt")), Status: "completed", GatesPassed: true, ProtectionPassed: true,
					Metrics: map[string]float64{"acceptance": acceptance, "cost": 0, "duration": 0}}
				data, _ := json.Marshal(receipt)
				_, err = tx.Exec(f.ctx, `INSERT INTO pangu.evaluation_experiment_receipts(experiment_id,generation,arm,case_id,repeat,attempt,receipt,receipt_hash)
   VALUES($1,$2,$3,$4,$5,1,$6,$7)`, e.ID, e.Generation, arm, c.ID, repeat, data, jsonHash(receipt))
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
}
