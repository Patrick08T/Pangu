package evaluation

import (
	"strings"
	"testing"
)

func experimentFixture(s *FrozenSuite) ExperimentSpec {
	return ExperimentSpec{ID: "experiment", Kind: "role_change", Purpose: "evaluation",
		LogicalProjectID: s.ProjectID, SuiteID: s.ID, SuiteHash: s.Hash,
		EnvironmentID: "environment", EnvironmentHash: strings.Repeat("a", 64),
		PolicyRevision: 1, Generation: 1, BaselineVersion: "baseline-version",
		CandidateHash: strings.Repeat("b", 64), SourceBaseline: strings.Repeat("c", 64),
		ConfigHash: strings.Repeat("d", 64), Budget: ExperimentBudget{MaxCalls: 10000, MaxTokens: 1000000},
		Baseline:  ExperimentArm{RoleHash: strings.Repeat("e", 64), CompiledHash: strings.Repeat("f", 64), MemoryHash: EmptyMemoryHash},
		Candidate: ExperimentArm{RoleHash: strings.Repeat("b", 64), CompiledHash: strings.Repeat("1", 64), MemoryHash: EmptyMemoryHash}}
}

func frozenFixture(t *testing.T, n int) (*FrozenSuite, *FrozenExperiment) {
	t.Helper()
	s, err := FreezeSuite(suiteFixture(n))
	if err != nil {
		t.Fatal(err)
	}
	e, err := FreezeExperiment(s, experimentFixture(s))
	if err != nil {
		t.Fatal(err)
	}
	return s, e
}

func receiptFixture(t *testing.T, s *FrozenSuite, e *FrozenExperiment) ExperimentReceipt {
	t.Helper()
	hash, err := ExecutionInputHash(s, e, "candidate", s.Cases[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	return ExperimentReceipt{ExperimentID: e.ID, ExperimentHash: e.Hash, Generation: e.Generation,
		Arm: "candidate", CaseID: s.Cases[0].ID, Repeat: 1, Attempt: 1,
		ExecutionInputHash: hash, EnvironmentHash: e.EnvironmentHash, CompiledHash: e.Candidate.CompiledHash,
		CheckerHash: s.Cases[0].CheckerHash, EvidenceHash: strings.Repeat("2", 64), Status: "completed",
		GatesPassed: true, ProtectionPassed: true, Metrics: map[string]float64{"acceptance": 1}}
}

func TestExperimentSeparatesRoleAndMemoryBenefits(t *testing.T) {
	s, _ := FreezeSuite(suiteFixture(4))
	spec := experimentFixture(s)
	if _, err := FreezeExperiment(s, spec); err != nil {
		t.Fatal(err)
	}
	spec.Candidate.MemoryHash = strings.Repeat("3", 64)
	if _, err := FreezeExperiment(s, spec); err == nil {
		t.Fatal("mixed role and memory changes")
	}
	spec.Kind = "memory_benefit"
	if _, err := FreezeExperiment(s, spec); err == nil {
		t.Fatal("memory experiment changed role")
	}
	spec.Candidate.RoleHash = spec.Baseline.RoleHash
	if _, err := FreezeExperiment(s, spec); err != nil {
		t.Fatal(err)
	}
	spec.Baseline.MemoryHash = spec.Candidate.MemoryHash
	if _, err := FreezeExperiment(s, spec); err == nil {
		t.Fatal("memory control is not empty")
	}
}

func TestReceiptRejectsIdentityDriftAndUnplannedRetries(t *testing.T) {
	s, e := frozenFixture(t, 4)
	if err := ValidateReceipt(s, e, receiptFixture(t, s, e)); err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(*ExperimentReceipt){
		"generation":  func(r *ExperimentReceipt) { r.Generation++ },
		"experiment":  func(r *ExperimentReceipt) { r.ExperimentID = "other" },
		"arm":         func(r *ExperimentReceipt) { r.Arm = "baseline" },
		"input":       func(r *ExperimentReceipt) { r.ExecutionInputHash = strings.Repeat("0", 64) },
		"environment": func(r *ExperimentReceipt) { r.EnvironmentHash = strings.Repeat("0", 64) },
		"compiled":    func(r *ExperimentReceipt) { r.CompiledHash = strings.Repeat("0", 64) },
		"checker":     func(r *ExperimentReceipt) { r.CheckerHash = strings.Repeat("0", 64) },
		"retry":       func(r *ExperimentReceipt) { r.Attempt = 2 },
		"repeat":      func(r *ExperimentReceipt) { r.Repeat = 3 },
		"development": func(r *ExperimentReceipt) { r.CaseID = s.Cases[len(s.Cases)-1].ID },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			r := receiptFixture(t, s, e)
			change(&r)
			if err := ValidateReceipt(s, e, r); err == nil {
				t.Fatal("accepted mismatched receipt")
			}
		})
	}
}
