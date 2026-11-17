package evaluation

// SuiteSpec is registered by the independent evaluator, never by a candidate.
// Cases contain references and hashes only; hidden inputs/answers stay in the
// retained case store. Every numeric decision parameter is explicit.
type SuiteSpec struct {
	ID        string        `json:"id"`
	ProjectID string        `json:"project_id"`
	Version   int           `json:"version"`
	Cases     []SuiteCase   `json:"cases"`
	Rules     DecisionRules `json:"rules"`
}

type SuiteCase struct {
	ID           string `json:"id"`
	FamilyID     string `json:"family_id"`
	TaskID       string `json:"task_id"`
	SourceRunID  string `json:"source_run_id"`
	InputHash    string `json:"input_hash"`
	ExpectedHash string `json:"expected_hash"`
	CriteriaHash string `json:"criteria_hash"`
	CheckerHash  string `json:"checker_hash"`
	Partition    string `json:"partition"`
	Group        string `json:"group"`
	Critical     bool   `json:"critical"`
}

type DecisionRules struct {
	Method         string       `json:"method"`
	FailurePolicy  string       `json:"failure_policy"`
	Confidence     float64      `json:"confidence"`
	MinFamilies    int          `json:"min_families"`
	Repeats        int          `json:"repeats"`
	MaxCandidates  int          `json:"max_candidates"`
	Primary        string       `json:"primary"`
	MinImprovement float64      `json:"min_improvement"`
	Metrics        []MetricRule `json:"metrics"`
	Groups         []string     `json:"groups"`
}

// Bounds apply to individual observations, before oriented paired differences.
// Tolerance is an absolute non-inferiority margin in the metric's own units.
type MetricRule struct {
	Name         string  `json:"name"`
	HigherBetter bool    `json:"higher_better"`
	Lower        float64 `json:"lower"`
	Upper        float64 `json:"upper"`
	Tolerance    float64 `json:"tolerance"`
}

type FrozenSuite struct {
	SuiteSpec
	Hash        string `json:"hash"`
	HoldoutHash string `json:"holdout_hash"`
}

type ExperimentSpec struct {
	ID               string           `json:"id"`
	Kind             string           `json:"kind"`
	Purpose          string           `json:"purpose"`
	LogicalProjectID string           `json:"logical_project_id"`
	EnvironmentID    string           `json:"environment_id"`
	EnvironmentHash  string           `json:"environment_hash"`
	SuiteID          string           `json:"suite_id"`
	SuiteHash        string           `json:"suite_hash"`
	PolicyRevision   int              `json:"policy_revision"`
	Generation       int              `json:"generation"`
	BaselineVersion  string           `json:"baseline_version"`
	CandidateHash    string           `json:"candidate_hash"`
	SourceBaseline   string           `json:"source_baseline"`
	ConfigHash       string           `json:"config_hash"`
	Budget           ExperimentBudget `json:"budget"`
	Baseline         ExperimentArm    `json:"baseline"`
	Candidate        ExperimentArm    `json:"candidate"`
}

type ExperimentBudget struct {
	MaxCalls  int   `json:"max_calls"`
	MaxTokens int64 `json:"max_tokens"`
}

type ExperimentArm struct {
	RoleHash     string `json:"role_hash"`
	CompiledHash string `json:"compiled_hash"`
	MemoryHash   string `json:"memory_hash"`
}

type FrozenExperiment struct {
	ExperimentSpec
	Hash string `json:"hash"`
}

// Receipt must come from a trusted checker and match the dispatch identity.
// Attempt is one in the preregistered invalidate-on-execution-error policy.
type ExperimentReceipt struct {
	ExperimentID       string             `json:"experiment_id"`
	ExperimentHash     string             `json:"experiment_hash"`
	Generation         int                `json:"generation"`
	Arm                string             `json:"arm"`
	CaseID             string             `json:"case_id"`
	Repeat             int                `json:"repeat"`
	Attempt            int                `json:"attempt"`
	ExecutionInputHash string             `json:"execution_input_hash"`
	EnvironmentHash    string             `json:"environment_hash"`
	CompiledHash       string             `json:"compiled_hash"`
	CheckerHash        string             `json:"checker_hash"`
	EvidenceHash       string             `json:"evidence_hash"`
	Status             string             `json:"status"`
	GatesPassed        bool               `json:"gates_passed"`
	ProtectionPassed   bool               `json:"protection_passed"`
	Metrics            map[string]float64 `json:"metrics"`
}

type MetricInterval struct {
	Metric   string  `json:"metric"`
	Group    string  `json:"group"`
	Families int     `json:"families"`
	Mean     float64 `json:"mean"`
	Lower    float64 `json:"lower"`
	Upper    float64 `json:"upper"`
}

// Disclosure deliberately excludes hidden case IDs, observations and answers.
type ExperimentDecision struct {
	DecisionHash string           `json:"decision_hash,omitempty"`
	Outcome      string           `json:"outcome"`
	Category     string           `json:"category"`
	Intervals    []MetricInterval `json:"-"`
}
