package experience

import (
	"errors"
	"testing"
)

func TestIntentRetryAdvancesGenerationAndFencesPreviousReceipt(t *testing.T) {
	intent, err := NewLearningIntent(IntentInput{SourceRunID: "run", EvidenceRevision: 1,
		PolicyRevision: 1, Purpose: "business", Terminal: true, StopConfirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	intent.Stage, intent.Retryable, intent.Receipt = "failed", true, "old"
	next, err := RetryIntent(intent, 1)
	if err != nil || next.Generation != 2 || next.Stage != "pending" || next.Receipt != "" || next.IdempotencyKey == intent.IdempotencyKey {
		t.Fatalf("retry did not fence old generation: %+v %v", next, err)
	}
	if _, err := ApplyIntentReceipt(next, 1, "old", "completed"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale generation completed retry")
	}
	intent.Retryable = false
	if _, err := RetryIntent(intent, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("nonretryable failure retried")
	}
}
