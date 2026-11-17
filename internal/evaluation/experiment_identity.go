package evaluation

// EmptyMemoryHash is SHA-256 of the canonical empty frozen memory list ([]).
const EmptyMemoryHash = "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"

func FreezeExperiment(s *FrozenSuite, spec ExperimentSpec) (*FrozenExperiment, error) {
	if s == nil {
		return nil, ErrInvalid
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if spec.ID == "" || spec.EnvironmentID == "" || spec.BaselineVersion == "" {
		return nil, ErrInvalid
	}
	if spec.LogicalProjectID != s.ProjectID || spec.SuiteID != s.ID || spec.SuiteHash != s.Hash {
		return nil, ErrConflict
	}
	if spec.Purpose != "evaluation" || spec.PolicyRevision < 1 || spec.Generation < 1 {
		return nil, ErrInvalid
	}
	if spec.Budget.MaxCalls < 1 || spec.Budget.MaxTokens < 1 {
		return nil, ErrInvalid
	}
	hashes := []string{spec.EnvironmentHash, spec.CandidateHash, spec.SourceBaseline, spec.ConfigHash,
		spec.Baseline.RoleHash, spec.Baseline.CompiledHash, spec.Baseline.MemoryHash,
		spec.Candidate.RoleHash, spec.Candidate.CompiledHash, spec.Candidate.MemoryHash}
	for _, hash := range hashes {
		if !validHash(hash) {
			return nil, ErrInvalid
		}
	}
	switch spec.Kind {
	case "role_change":
		if spec.Baseline.MemoryHash != spec.Candidate.MemoryHash || spec.Baseline.RoleHash == spec.Candidate.RoleHash {
			return nil, ErrInvalid
		}
	case "memory_benefit":
		if spec.Baseline.RoleHash != spec.Candidate.RoleHash || spec.Baseline.MemoryHash != EmptyMemoryHash || spec.Candidate.MemoryHash == EmptyMemoryHash {
			return nil, ErrInvalid
		}
	default:
		return nil, ErrInvalid
	}
	return &FrozenExperiment{ExperimentSpec: spec, Hash: contentHash(spec)}, nil
}

func validateExperiment(s *FrozenSuite, e *FrozenExperiment) error {
	if e == nil {
		return ErrInvalid
	}
	copy, err := FreezeExperiment(s, e.ExperimentSpec)
	if err != nil {
		return err
	}
	if copy.Hash != e.Hash {
		return ErrConflict
	}
	return nil
}

func ExecutionInputHash(s *FrozenSuite, e *FrozenExperiment, arm, caseID string, repeat int) (string, error) {
	if err := validateExperiment(s, e); err != nil {
		return "", err
	}
	return executionInputHash(s, e, arm, caseID, repeat)
}

func executionInputHash(s *FrozenSuite, e *FrozenExperiment, arm, caseID string, repeat int) (string, error) {
	if arm != "baseline" && arm != "candidate" || repeat < 1 || repeat > s.Rules.Repeats {
		return "", ErrInvalid
	}
	for _, c := range s.Cases {
		if c.ID == caseID && c.Partition != "development" {
			return contentHash(struct {
				Experiment, Arm, Case, Input string
				Repeat                       int
			}{e.Hash, arm, caseID, c.InputHash, repeat}), nil
		}
	}
	return "", ErrNotFound
}

func ValidateReceipt(s *FrozenSuite, e *FrozenExperiment, r ExperimentReceipt) error {
	if err := validateExperiment(s, e); err != nil {
		return err
	}
	return validateReceipt(s, e, r)
}

func validateReceipt(s *FrozenSuite, e *FrozenExperiment, r ExperimentReceipt) error {
	if r.ExperimentID != e.ID || r.ExperimentHash != e.Hash || r.Generation != e.Generation {
		return ErrConflict
	}
	if r.EnvironmentHash != e.EnvironmentHash || r.Attempt != 1 || !validHash(r.EvidenceHash) {
		return ErrConflict
	}
	input, err := executionInputHash(s, e, r.Arm, r.CaseID, r.Repeat)
	if err != nil || input != r.ExecutionInputHash {
		return ErrConflict
	}
	compiled := e.Baseline.CompiledHash
	if r.Arm == "candidate" {
		compiled = e.Candidate.CompiledHash
	}
	if compiled != r.CompiledHash {
		return ErrConflict
	}
	for _, c := range s.Cases {
		if c.ID == r.CaseID && c.CheckerHash != r.CheckerHash {
			return ErrConflict
		}
	}
	if r.Status != "completed" && r.Status != "failed" && r.Status != "timeout" && r.Status != "infrastructure_error" {
		return ErrInvalid
	}
	registered := make(map[string]bool, len(s.Rules.Metrics))
	for _, metric := range s.Rules.Metrics {
		registered[metric.Name] = true
	}
	for name, value := range r.Metrics {
		if !registered[name] || !finite(value) {
			return ErrInvalid
		}
	}
	return nil
}
