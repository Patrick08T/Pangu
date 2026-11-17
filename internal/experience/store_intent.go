package experience

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type IntentReceipt struct {
	Generation int64  `json:"generation"`
	ReceiptID  string `json:"receipt_id"`
	Stage      string `json:"stage"`
	LastError  string `json:"last_error"`
	Retryable  bool   `json:"retryable"`
}

func (s Store) EnsureLearningIntent(ctx context.Context, scope Scope, input IntentInput) (LearningIntent, error) {
	return withStoreTx(ctx, s, scope, "intent_write", func(tx pgx.Tx, scope Scope) (LearningIntent, error) {
		intent, err := NewLearningIntent(input)
		if err != nil {
			return LearningIntent{}, err
		}
		if err = s.authorizedIntentTx(ctx, tx, scope, input); err != nil {
			return LearningIntent{}, err
		}
		var existing LearningIntent
		err = scanJSON(tx.QueryRow(ctx, `SELECT intent FROM pangu.experience_learning_intents WHERE namespace=$1 AND project_id=$2 AND source_run_id=$3 AND evidence_revision=$4 AND policy_revision=$5`, scope.Namespace, scope.ProjectID, input.SourceRunID, input.EvidenceRevision, input.PolicyRevision), &existing)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return LearningIntent{}, err
		}
		intent.ID = uuid.NewString()
		data, err := json.Marshal(intent)
		if err != nil {
			return LearningIntent{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_learning_intents(id,namespace,project_id,source_run_id,evidence_revision,policy_revision,intent) VALUES($1,$2,$3,$4,$5,$6,$7)`, intent.ID, scope.Namespace, scope.ProjectID, input.SourceRunID, input.EvidenceRevision, input.PolicyRevision, data)
		if err != nil {
			return LearningIntent{}, err
		}
		return intent, auditTx(ctx, tx, scope, intent.ID, map[string]any{"generation": intent.Generation, "stage": intent.Stage})
	})
}

func (s Store) authorizedIntentTx(ctx context.Context, tx pgx.Tx, scope Scope, input IntentInput) error {
	if s.VerifyIntent == nil {
		return ErrForbidden
	}
	if err := s.VerifyIntent(ctx, tx, scope, input); err != nil {
		return err
	}
	policy, err := policyTx(ctx, tx, scope)
	if err != nil {
		return err
	}
	if !policy.Enabled || policy.Revoked || policy.Revision != input.PolicyRevision {
		return ErrForbidden
	}
	return nil
}

func readIntentTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (LearningIntent, error) {
	var intent LearningIntent
	err := scanJSON(tx.QueryRow(ctx, `SELECT intent FROM pangu.experience_learning_intents WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, id), &intent)
	return intent, err
}

func writeIntentTx(ctx context.Context, tx pgx.Tx, scope Scope, intent LearningIntent) error {
	data, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.experience_learning_intents SET intent=$4,updated_at=now() WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, intent.ID, data)
	if err != nil {
		return err
	}
	return auditTx(ctx, tx, scope, intent.ID, map[string]any{"generation": intent.Generation, "stage": intent.Stage, "receipt": intent.Receipt})
}

func (s Store) CompleteLearningIntent(ctx context.Context, scope Scope, id string, receipt IntentReceipt) (LearningIntent, error) {
	return withStoreTx(ctx, s, scope, "intent_complete", func(tx pgx.Tx, scope Scope) (LearningIntent, error) {
		intent, err := readIntentTx(ctx, tx, scope, id)
		if err != nil {
			return LearningIntent{}, err
		}
		if err = s.authorizedIntentTx(ctx, tx, scope, intent.Input); err != nil {
			return LearningIntent{}, err
		}
		if len(receipt.LastError) > 1024 || diagnostic.Sensitive([]byte(receipt.LastError)) {
			return LearningIntent{}, ErrInvalid
		}
		next, err := ApplyIntentReceipt(intent, receipt.Generation, receipt.ReceiptID, receipt.Stage)
		if err != nil {
			return LearningIntent{}, err
		}
		if intent.Receipt != "" && (intent.LastError != receipt.LastError || intent.Retryable != receipt.Retryable) {
			return LearningIntent{}, ErrConflict
		}
		next.LastError, next.Retryable = receipt.LastError, receipt.Retryable
		return next, writeIntentTx(ctx, tx, scope, next)
	})
}

func (s Store) RetryLearningIntent(ctx context.Context, scope Scope, id string, generation int64) (LearningIntent, error) {
	return withStoreTx(ctx, s, scope, "intent_retry", func(tx pgx.Tx, scope Scope) (LearningIntent, error) {
		intent, err := readIntentTx(ctx, tx, scope, id)
		if err != nil {
			return LearningIntent{}, err
		}
		if err = s.authorizedIntentTx(ctx, tx, scope, intent.Input); err != nil {
			return LearningIntent{}, err
		}
		next, err := RetryIntent(intent, generation)
		if err != nil {
			return LearningIntent{}, err
		}
		return next, writeIntentTx(ctx, tx, scope, next)
	})
}
