package learning

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type claimRequest struct {
	ID, VersionID, IntentID, CheckerID, SourceSetHash string
	Generation                                        int64
}

// CheckClaim freezes a request; only Activate can atomically commit a passing receipt.
func CheckClaim(ctx context.Context, scope experience.Scope, versionID, checkerID, intentID string, generation int64) (experience.Activation, error) {
	return transaction(ctx, scope, "claim_check", func(tx pgx.Tx, scope experience.Scope) (experience.Activation, error) {
		out := experience.Activation{IntentID: intentID, Generation: generation}
		version, err := NewStore().ReadTx(ctx, tx, scope, versionID)
		if err != nil {
			return out, err
		}
		request := claimRequest{ID: uuid.NewString(), VersionID: versionID, IntentID: intentID,
			CheckerID: checkerID, Generation: generation, SourceSetHash: jsonHash(version.Sources)}
		if _, err = claimBindingsTx(ctx, tx, scope, version, request); err != nil {
			return out, err
		}
		err = tx.QueryRow(ctx, `INSERT INTO pangu.learning_claim_requests(id,namespace,project_id,version_id,intent_id,generation,checker_id,source_set_hash)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(version_id,intent_id,generation,checker_id) DO NOTHING RETURNING id`,
			request.ID, scope.Namespace, scope.ProjectID, versionID, intentID, generation, checkerID, request.SourceSetHash).Scan(&out.ReceiptID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT id FROM pangu.learning_claim_requests WHERE namespace=$1 AND project_id=$2
			 AND version_id=$3 AND intent_id=$4 AND generation=$5 AND checker_id=$6`, scope.Namespace, scope.ProjectID,
				versionID, intentID, generation, checkerID).Scan(&out.ReceiptID)
		}
		if err != nil {
			return out, err
		}
		return out, auditTx(ctx, tx, scope, out.ReceiptID, map[string]string{"version_id": versionID, "checker_id": checkerID})
	})
}

func claimBindingsTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, request claimRequest) (Checker, error) {
	checker, err := checkerTx(ctx, tx, scope, request.CheckerID)
	if err != nil {
		return checker, err
	}
	if checker.Spec.ContentHash != version.BodyHash || request.SourceSetHash != jsonHash(version.Sources) {
		return checker, experience.ErrForbidden
	}
	var raw []byte
	var intentCreated, versionCreated time.Time
	err = tx.QueryRow(ctx, `SELECT i.intent,i.created_at,v.created_at FROM pangu.experience_learning_intents i
	 JOIN pangu.experience_versions v ON v.id=$4 AND v.namespace=i.namespace AND v.project_id=i.project_id
	 WHERE i.namespace=$1 AND i.project_id=$2 AND i.id=$3`, scope.Namespace, scope.ProjectID, request.IntentID, version.ID).
		Scan(&raw, &intentCreated, &versionCreated)
	if errors.Is(err, pgx.ErrNoRows) {
		return checker, experience.ErrForbidden
	}
	if err != nil {
		return checker, err
	}
	var intent experience.LearningIntent
	if err = json.Unmarshal(raw, &intent); err != nil {
		return checker, err
	}
	if intent.Generation != request.Generation || intent.Stage == "failed" || intent.Stage == "cancelled" ||
		!checker.CreatedAt.Before(intentCreated) || !checker.CreatedAt.Before(versionCreated) {
		return checker, experience.ErrForbidden
	}
	if err = verifyIntent(ctx, tx, scope, intent.Input); err != nil {
		return checker, err
	}
	var bound bool
	for _, source := range version.Sources {
		bound = bound || source.RunID == intent.Input.SourceRunID
	}
	if !bound {
		return checker, experience.ErrForbidden
	}
	return checker, nil
}

func readClaimRequest(ctx context.Context, tx pgx.Tx, scope experience.Scope, id string) (claimRequest, error) {
	var out claimRequest
	if !validID(id) {
		return out, experience.ErrForbidden
	}
	err := tx.QueryRow(ctx, `SELECT id,version_id,intent_id,generation,checker_id,source_set_hash FROM pangu.learning_claim_requests
	 WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, id).
		Scan(&out.ID, &out.VersionID, &out.IntentID, &out.Generation, &out.CheckerID, &out.SourceSetHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, experience.ErrForbidden
	}
	return out, err
}
