package catalog

// ExperimentIdentity 只从服务端不可变实验记录读取；不是客户端运行选项。
type ExperimentIdentity struct {
	ID               string        `json:"id"`
	Kind             string        `json:"kind"`
	Purpose          string        `json:"purpose"`
	LogicalProjectID string        `json:"logical_project_id"`
	EnvironmentID    string        `json:"environment_id"`
	EnvironmentHash  string        `json:"environment_hash"`
	PolicyRevision   int           `json:"policy_revision"`
	Generation       int           `json:"generation"`
	BaselineVersion  string        `json:"baseline_version"`
	CandidateHash    string        `json:"candidate_hash"`
	SourceBaseline   string        `json:"source_baseline"`
	ConfigHash       string        `json:"config_hash"`
	Baseline         ExperimentArm `json:"baseline"`
	Candidate        ExperimentArm `json:"candidate"`
	Hash             string        `json:"-"`
	DecisionHash     string        `json:"-"`
}

type ExperimentArm struct {
	RoleHash     string `json:"role_hash"`
	CompiledHash string `json:"compiled_hash"`
	MemoryHash   string `json:"memory_hash"`
}

func verifyPrepared(e ExperimentIdentity, p *PreparedCandidate) error {
	if p == nil || e.Kind != "role_change" || e.Purpose != "evaluation" || e.CandidateHash != p.CandidateHash || e.Baseline.RoleHash != p.BaselineHash || e.Candidate.RoleHash != p.CandidateHash || e.Baseline.CompiledHash != p.BaselineCompiledHash || e.Candidate.CompiledHash != p.CandidateCompiledHash || e.Baseline.MemoryHash != e.Candidate.MemoryHash {
		return ErrConflict
	}
	return nil
}
