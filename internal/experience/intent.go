package experience

import (
	"encoding/json"
	"slices"
)

type IntentInput struct {
	SourceRunID      string `json:"source_run_id"`
	EvidenceRevision int64  `json:"evidence_revision"`
	PolicyRevision   int64  `json:"policy_revision"`
	Purpose          string `json:"purpose"`
	Terminal         bool   `json:"terminal"`
	StopConfirmed    bool   `json:"stop_confirmed"`
}

type LearningIntent struct {
	ID             string      `json:"id"`
	Input          IntentInput `json:"input"`
	Generation     int64       `json:"generation"`
	IdempotencyKey string      `json:"idempotency_key"`
	Stage          string      `json:"stage"`
	Receipt        string      `json:"receipt"`
	LastError      string      `json:"last_error"`
	Retryable      bool        `json:"retryable"`
}

func RetryIntent(intent LearningIntent, expected int64) (LearningIntent, error) {
	if intent.Generation != expected || intent.Stage != "failed" || !intent.Retryable {
		return LearningIntent{}, ErrConflict
	}
	intent.Generation++
	intent.Stage, intent.Receipt, intent.LastError, intent.Retryable = "pending", "", "", false
	data, _ := json.Marshal(struct {
		Input      IntentInput
		Generation int64
	}{intent.Input, intent.Generation})
	intent.IdempotencyKey = hashBytes(data)
	return intent, nil
}

func NewLearningIntent(input IntentInput) (LearningIntent, error) {
	if input.Purpose != "business" || !input.Terminal || !input.StopConfirmed ||
		input.SourceRunID == "" || input.EvidenceRevision < 1 || input.PolicyRevision < 1 {
		return LearningIntent{}, ErrInvalid
	}
	data, _ := json.Marshal(input)
	return LearningIntent{Input: input, Generation: 1, Stage: "pending", IdempotencyKey: hashBytes(data)}, nil
}

func ApplyIntentReceipt(intent LearningIntent, generation int64, receipt, stage string) (LearningIntent, error) {
	if generation != intent.Generation || receipt == "" ||
		!slices.Contains([]string{"completed", "failed", "cancelled"}, stage) ||
		(intent.Receipt != "" && (intent.Receipt != receipt || intent.Stage != stage)) {
		return LearningIntent{}, ErrConflict
	}
	intent.Receipt, intent.Stage = receipt, stage
	return intent, nil
}
