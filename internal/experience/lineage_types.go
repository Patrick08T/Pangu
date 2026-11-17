package experience

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// RunVerifier resolves a server-owned request/run binding using this transaction.
type RunVerifier func(context.Context, pgx.Tx, Scope, RunLineageQuery) (RunBinding, error)

type RunBinding struct {
	Namespace           string `json:"-"`
	ProjectID           string `json:"-"`
	ActualRunID         string `json:"actual_run_id"`
	RequestID           string `json:"-"`
	WatermarkGeneration int64  `json:"watermark_generation"`
}

type RunLineageQuery struct {
	ActualRunID         string
	RequestID           string
	WatermarkGeneration int64
	UsagePage           Page
	ArtifactPage        Page
}

type AuditState struct {
	Status        string `json:"status"`
	Proven        bool   `json:"proven"`
	InvalidReason string `json:"invalid_reason,omitempty"`
	Redacted      bool   `json:"redacted"`
}

type WatermarkSummary struct {
	AuditState
	Present        bool        `json:"present"`
	Generation     int64       `json:"generation"`
	PolicyRevision int64       `json:"policy_revision"`
	Hash           string      `json:"hash,omitempty"`
	References     []Reference `json:"references"`
}

type RunUsageSummary struct {
	AuditState
	NodeID             string      `json:"node_id"`
	Iteration          int         `json:"iteration"`
	ApprovalGeneration int64       `json:"approval_generation"`
	ApprovalRevision   int64       `json:"approval_revision"`
	MatchReason        string      `json:"match_reason"`
	InjectionHash      string      `json:"injection_hash,omitempty"`
	References         []Reference `json:"references"`
	CreatedAt          time.Time   `json:"created_at"`
}

type ArtifactSummary struct {
	AuditState
	ID                 string              `json:"id"`
	NodeID             string              `json:"node_id"`
	Kind               string              `json:"kind"`
	OutputHash         string              `json:"output_hash,omitempty"`
	InputHash          string              `json:"input_hash,omitempty"`
	ExecutionInputHash string              `json:"execution_input_hash,omitempty"`
	Generation         int64               `json:"generation"`
	References         []Reference         `json:"references"`
	Parents            []ArtifactReference `json:"parents"`
}

type RunUsagePage struct {
	Items      []RunUsageSummary `json:"items"`
	Total      int64             `json:"total"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type ArtifactSummaryPage struct {
	Items      []ArtifactSummary `json:"items"`
	Total      int64             `json:"total"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type RunLineage struct {
	ActualRunID string              `json:"actual_run_id"`
	Watermark   WatermarkSummary    `json:"watermark"`
	Usages      RunUsagePage        `json:"usages"`
	Artifacts   ArtifactSummaryPage `json:"artifacts"`
	// Artifacts are paginated run roots; Parents can be expanded separately.
	ChainComplete bool `json:"chain_complete"`
}
