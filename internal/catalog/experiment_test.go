package catalog

import (
	"errors"
	"testing"
)

func TestExperimentCandidateRejectsAnyFrozenHashDrift(t *testing.T) {
	p := &PreparedCandidate{BaselineHash: "base", CandidateHash: "next", BaselineCompiledHash: "before", CandidateCompiledHash: "after"}
	e := ExperimentIdentity{Kind: "role_change", Purpose: "evaluation", CandidateHash: "next",
		Baseline:  ExperimentArm{RoleHash: "base", CompiledHash: "before", MemoryHash: "same"},
		Candidate: ExperimentArm{RoleHash: "next", CompiledHash: "after", MemoryHash: "same"}}
	if err := verifyPrepared(e, p); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*ExperimentIdentity){
		"kind":              func(v *ExperimentIdentity) { v.Kind = "memory_benefit" },
		"purpose":           func(v *ExperimentIdentity) { v.Purpose = "production" },
		"candidate":         func(v *ExperimentIdentity) { v.CandidateHash = "forged" },
		"baseline role":     func(v *ExperimentIdentity) { v.Baseline.RoleHash = "forged" },
		"candidate role":    func(v *ExperimentIdentity) { v.Candidate.RoleHash = "forged" },
		"baseline compile":  func(v *ExperimentIdentity) { v.Baseline.CompiledHash = "forged" },
		"candidate compile": func(v *ExperimentIdentity) { v.Candidate.CompiledHash = "forged" },
		"memory":            func(v *ExperimentIdentity) { v.Candidate.MemoryHash = "different" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := e
			mutate(&changed)
			if err := verifyPrepared(changed, p); !errors.Is(err, ErrConflict) {
				t.Fatalf("frozen drift accepted: %v", err)
			}
		})
	}
}
