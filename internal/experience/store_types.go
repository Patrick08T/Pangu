package experience

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type Scope struct{ Namespace, ProjectID, ActorID, Action string }
type Authorizer func(context.Context, pgx.Tx, Scope) error
type SourceVerifier func(context.Context, pgx.Tx, Scope, Source) error
type ClaimVerifier func(context.Context, pgx.Tx, Scope, Version, Activation) error
type BenefitVerifier func(context.Context, pgx.Tx, Scope, Version, DeliveryChange) error
type IntentVerifier func(context.Context, pgx.Tx, Scope, IntentInput) error

// Store 不持有配置正文缓存；所有授权和来源校验在当前数据库事务内执行。
type Store struct {
	Authorize       Authorizer
	VerifySource    SourceVerifier
	VerifyClaim     ClaimVerifier
	VerifyBenefit   BenefitVerifier
	VerifyIntent    IntentVerifier
	VerifyRun       RunVerifier
	frozenEmptyOnly bool
}

type Activation struct {
	IntentID   string `json:"intent_id"`
	Generation int64  `json:"generation"`
	ReceiptID  string `json:"receipt_id"`
}

type DeliveryChange struct {
	ExpectedRevision int64          `json:"expected_revision"`
	Policy           DeliveryPolicy `json:"policy"`
	EvidenceID       string         `json:"evidence_id"`
}

type FrozenVersion struct {
	Reference Reference      `json:"reference"`
	Delivery  string         `json:"delivery"`
	Canary    DeliveryPolicy `json:"canary"`
}

type Watermark struct {
	RunID          string          `json:"run_id"`
	Generation     int64           `json:"generation"`
	PolicyRevision int64           `json:"policy_revision"`
	Versions       []FrozenVersion `json:"versions"`
	Hash           string          `json:"hash"`
}

type Usage struct {
	Input         SelectionInput `json:"input"`
	WatermarkHash string         `json:"watermark_hash"`
	References    []Reference    `json:"references"`
	InjectionHash string         `json:"injection_hash"`
	MatchReason   string         `json:"match_reason"`
	CreatedAt     time.Time      `json:"created_at"`
}
