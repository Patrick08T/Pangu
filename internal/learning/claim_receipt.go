package learning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

type ClaimReceipt struct {
	ID            string    `json:"id"`
	Hash          string    `json:"hash"`
	CheckerHash   string    `json:"checker_hash"`
	SourceSetHash string    `json:"source_set_hash"`
	Passed        bool      `json:"passed"`
	CreatedAt     time.Time `json:"created_at"`
}

func ReadReceipt(ctx context.Context, scope experience.Scope, id string) (ClaimReceipt, error) {
	return transaction(ctx, scope, "receipt_read", func(tx pgx.Tx, scope experience.Scope) (ClaimReceipt, error) {
		var out ClaimReceipt
		request, err := readClaimRequest(ctx, tx, scope, id)
		if err != nil {
			return out, err
		}
		version, err := NewStore().ReadTx(ctx, tx, scope, request.VersionID)
		if err != nil {
			return out, err
		}
		if _, err = claimBindingsTx(ctx, tx, scope, version, request); err != nil {
			return out, err
		}
		err = tx.QueryRow(ctx, `SELECT id,receipt_hash,checker_hash,source_set_hash,passed,created_at FROM pangu.learning_claim_receipts WHERE id=$1`, id).
			Scan(&out.ID, &out.Hash, &out.CheckerHash, &out.SourceSetHash, &out.Passed, &out.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return out, experience.ErrNotFound
		}
		return out, err
	})
}

func computeClaimTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, activation experience.Activation) error {
	request, err := readClaimRequest(ctx, tx, scope, activation.ReceiptID)
	if err != nil {
		return err
	}
	if request.VersionID != version.ID || request.IntentID != activation.IntentID || request.Generation != activation.Generation {
		return experience.ErrForbidden
	}
	checker, err := claimBindingsTx(ctx, tx, scope, version, request)
	if err != nil {
		return err
	}
	if len(version.Sources) == 0 {
		return experience.ErrForbidden
	}
	for _, source := range version.Sources {
		if source.Kind != checker.Spec.SourceKind || source.NodeID != checker.Spec.NodeID {
			return experience.ErrForbidden
		}
		evidence, err := sourceTx(ctx, tx, scope, source, true)
		if err != nil {
			return err
		}
		if !checkJSON(checker.Spec, evidence.Body) {
			return experience.ErrForbidden
		}
	}
	hash := jsonHash(struct {
		Request     claimRequest
		CheckerHash string
	}{request, checker.Hash})
	_, err = tx.Exec(ctx, `INSERT INTO pangu.learning_claim_receipts(id,receipt_hash,checker_hash,source_set_hash,passed)
	 VALUES($1,$2,$3,$4,true) ON CONFLICT(id) DO NOTHING`, request.ID, hash, checker.Hash, request.SourceSetHash)
	if err != nil {
		return err
	}
	return auditTx(ctx, tx, scope, request.ID, map[string]string{"receipt_hash": hash})
}

func checkJSON(spec CheckerSpec, body []byte) bool {
	var value, expected any
	if decodeJSON(body, &value) != nil || decodeJSON(spec.Expected, &expected) != nil {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(spec.JSONPointer, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch current := value.(type) {
		case map[string]any:
			var exists bool
			value, exists = current[part]
			if !exists {
				return false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(current) || strconv.Itoa(index) != part {
				return false
			}
			value = current[index]
		default:
			return false
		}
	}
	return reflect.DeepEqual(value, expected)
}

func decodeJSON(body []byte, value any) error {
	if !json.Valid(body) {
		return experience.ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	return decoder.Decode(value)
}
