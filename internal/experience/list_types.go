package experience

import "time"

// Page 使用不透明keyset游标；Limit必须显式指定且在1..100之间。
type Page struct {
	AfterID string `json:"after_id,omitempty"`
	Limit   int    `json:"limit"`
}

type VersionSummary struct {
	ID               string    `json:"id"`
	EntryID          string    `json:"entry_id"`
	Revision         int64     `json:"revision"`
	Validity         string    `json:"validity"`
	Delivery         string    `json:"delivery"`
	DeliveryRevision int64     `json:"delivery_revision"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type VersionPage struct {
	Items      []VersionSummary `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type UsageSummary struct {
	RunID              string    `json:"run_id"`
	NodeID             string    `json:"node_id"`
	Iteration          int       `json:"iteration"`
	ApprovalGeneration int64     `json:"approval_generation"`
	ApprovalRevision   int64     `json:"approval_revision"`
	CreatedAt          time.Time `json:"created_at"`
}

type UsagePage struct {
	Items      []UsageSummary `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}
