package experience

import (
	"errors"
	"time"
)

var (
	ErrInvalid     = errors.New("经验数据或授权配置无效")
	ErrForbidden   = errors.New("经验操作未经授权")
	ErrNotFound    = errors.New("经验记录不存在")
	ErrConflict    = errors.New("经验修订或代际冲突")
	ErrInvalidated = errors.New("经验或成果依赖已失效")
)

type Applicability struct {
	Roles           []string `json:"roles"`
	RoleVersions    []string `json:"role_versions,omitempty"`
	Modules         []string `json:"modules,omitempty"`
	Keywords        []string `json:"keywords,omitempty"`
	ErrorSignatures []string `json:"error_signatures,omitempty"`
}

type Content struct {
	Kind            string        `json:"kind"`
	Observation     string        `json:"observation"`
	Practice        string        `json:"practice"`
	Conditions      string        `json:"conditions"`
	Counterexamples string        `json:"counterexamples"`
	Applicability   Applicability `json:"applicability"`
}

type Source struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	RunID     string `json:"run_id"`
	NodeID    string `json:"node_id"`
	ProjectID string `json:"project_id"`
	Hash      string `json:"hash"`
}

type Reference struct {
	VersionID        string `json:"version_id"`
	BodyHash         string `json:"body_hash"`
	DeliveryRevision int64  `json:"delivery_revision"`
}

type Version struct {
	ID               string         `json:"id"`
	EntryID          string         `json:"entry_id"`
	Revision         int64          `json:"revision"`
	ProjectID        string         `json:"project_id"`
	BodyHash         string         `json:"body_hash"`
	Content          Content        `json:"content"`
	Sources          []Source       `json:"sources"`
	Validity         string         `json:"validity"`
	Delivery         string         `json:"delivery"`
	DeliveryRevision int64          `json:"delivery_revision"`
	Canary           DeliveryPolicy `json:"canary"`
	ExpiresAt        time.Time      `json:"expires_at"`
	Tombstoned       bool           `json:"tombstoned"`
	InvalidReason    string         `json:"invalid_reason"`
}

type SelectionInput struct {
	ProjectID           string `json:"project_id"`
	RunID               string `json:"run_id"`
	NodeID              string `json:"node_id"`
	Round               int    `json:"round"`
	Generation          int64  `json:"generation"`
	WatermarkGeneration int64  `json:"watermark_generation,omitempty"`
	ApprovalRevision    int64  `json:"approval_revision"`
	BoundInputHash      string `json:"bound_input_hash"`
	RoleID              string `json:"role_id"`
	RoleVersion         string `json:"role_version"`
	Module              string `json:"module"`
	Text                string `json:"text"`
	ErrorText           string `json:"error_text"`
	CanaryScope         string `json:"canary_scope"`
}
