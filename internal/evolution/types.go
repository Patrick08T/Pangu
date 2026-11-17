package evolution

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
	"time"
)

type Scope struct {
	Namespace string
	ProjectID string
	ActorID   string
}
type EntryTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func (e EntryTarget) Key() string { return e.Kind + ":" + e.ID }

type EntrySnapshot struct {
	Target      EntryTarget `json:"target"`
	ProjectID   string      `json:"project_id"`
	VersionID   string      `json:"version_id"`
	Revision    int64       `json:"revision"`
	OwnerID     string      `json:"owner_id"`
	Enabled     bool        `json:"enabled"`
	RemoteState string      `json:"remote_state"`
}

// Entries由可信服务注入；必须在同tx校验管理权限、归属和修订，并复用既有远端对账。
type EntryAdapter interface {
	ReadTx(context.Context, pgx.Tx, Scope, EntryTarget) (EntrySnapshot, error)
	CompareAndSwapTx(context.Context, pgx.Tx, Scope, string, EntrySnapshot, string) (EntrySnapshot, error)
}

type Service struct {
	Memory   experience.Store
	Entries  EntryAdapter
	Evidence EvidenceReader
}

type CandidateRequest struct {
	Kind                   string                   `json:"kind,omitempty"`
	BenefitMemoryVersionID string                   `json:"benefit_memory_version_id,omitempty"`
	ID                     string                   `json:"id"`
	ProjectID              string                   `json:"project_id"`
	PolicyRevision         int64                    `json:"policy_revision"`
	AgentObjectID          string                   `json:"agent_object_id"`
	FlowObjectID           string                   `json:"flow_object_id"`
	BaselineVersion        string                   `json:"baseline_version"`
	EnvironmentID          string                   `json:"environment_id"`
	SuiteID                string                   `json:"suite_id"`
	Payload                catalog.CandidatePayload `json:"payload"`
	MemoryVersionIDs       []string                 `json:"memory_version_ids"`
	Entries                []EntryTarget            `json:"entries"`
	Budget                 experience.Budget        `json:"budget"`
}

type Intent struct {
	ID             string             `json:"id"`
	ProjectID      string             `json:"project_id"`
	PolicyRevision int64              `json:"policy_revision"`
	Generation     int                `json:"generation"`
	State          string             `json:"state"`
	RequestHash    string             `json:"request_hash"`
	ExperimentID   string             `json:"experiment_id"`
	ExperimentHash string             `json:"experiment_hash"`
	Targets        []string           `json:"targets"`
	Publication    *PublishedVersions `json:"publication,omitempty"`
	StartedAt      time.Time          `json:"started_at"`
}

type PublishedVersions struct {
	AgentID string `json:"agent_id"`
	FlowID  string `json:"flow_id,omitempty"`
}

type CaseKey struct {
	IntentID   string
	Generation int
	Arm        string
	CaseID     string
	Repeat     int
}

type ExecutionPlan struct {
	Key         CaseKey
	RequestID   string
	Experiment  evaluation.FrozenExperiment
	Environment Environment
	Definition  workflow.Definition
	InputHash   string
	Memories    []workflow.Experience
	MaxCalls    int64
	MaxTokens   int64
}

type MachineResult struct {
	RequestID          string
	RunID              string
	NodeID             string
	Text               string
	Status             string
	Calls              int64
	Tokens             int64
	Cost               float64
	Currency           string
	CostMeasured       bool
	TokensMeasured     bool
	DurationMeasured   bool
	DurationSeconds    float64
	Generation         int
	ExperimentHash     string
	ExecutionInputHash string
	InputTextHash      string
	EnvironmentHash    string
	CompiledHash       string
	RoleHash           string
	MemoryHash         string
	SourceHash         string
	StopConfirmed      bool
}

// EvidenceReader只能由可信scheduler服务注入，从实际submission/runtime/report计算，禁止HTTP传入。
type EvidenceReader interface {
	ReadExecutionTx(context.Context, pgx.Tx, Scope, string) (MachineResult, error)
	ReadObservationTx(context.Context, pgx.Tx, Scope, string) (BusinessEvidence, error)
}

// StopProof is runtime liveness evidence only, never a performance or scoring receipt.
type StopProof struct {
	RequestID          string `json:"request_id"`
	RunID              string `json:"run_id"`
	Status             string `json:"status"`
	Generation         int    `json:"generation"`
	ExperimentHash     string `json:"experiment_hash"`
	ExecutionInputHash string `json:"execution_input_hash"`
	SourceHash         string `json:"source_hash"`
	StopConfirmed      bool   `json:"stop_confirmed"`
}

// StopEvidenceReader must not require retained role, memory, checker, or output bodies.
type StopEvidenceReader interface {
	ReadStopTx(context.Context, pgx.Tx, Scope, string) (StopProof, error)
}

type BusinessEvidence struct {
	RunID             string
	BaselineRunID     string
	EntryKey          string
	VersionID         string
	BaselineVersionID string
	Purpose           string
	SourceHash        string
	Terminal          bool
	StopConfirmed     bool
	CandidateScore    float64
	BaselineScore     float64
	HardFailure       bool
}
