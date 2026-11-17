package workflow

import "encoding/json"

// OutputLineage points to a server-owned, transitively validated artifact record.
// It is metadata, not a claim that callers may use without checking the store.
type OutputLineage struct {
	Version            int    `json:"version"`
	ArtifactID         string `json:"artifact_id"`
	InputHash          string `json:"input_hash"`
	OutputHash         string `json:"output_hash"`
	ExecutionInputHash string `json:"execution_input_hash,omitempty"`
}

// MemoryReference identifies an immutable body and the authorized rollout revision.
type MemoryReference struct {
	VersionID        string `json:"version_id"`
	BodyHash         string `json:"body_hash"`
	DeliveryRevision int64  `json:"delivery_revision"`
}

type MemorySource struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	RunID        string `json:"run_id,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	EvidenceHash string `json:"evidence_hash"`
}

// Experience is task data, never a replacement for the role's trusted instruction.
// Content preserves the verified canonical JSON used by the experience store.
type Experience struct {
	Reference  MemoryReference `json:"reference"`
	Kind       string          `json:"kind"`
	Conditions string          `json:"conditions"`
	Content    json.RawMessage `json:"content"`
	Sources    []MemorySource  `json:"sources"`
	Reason     string          `json:"reason,omitempty"`
}
