package learning

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CheckerSpec supports a narrow, explicit JSON assertion, not arbitrary prose truth.
type CheckerSpec struct {
	Kind        string          `json:"kind"`
	ContentHash string          `json:"content_hash"`
	SourceKind  string          `json:"source_kind"`
	NodeID      string          `json:"node_id"`
	JSONPointer string          `json:"json_pointer"`
	Expected    json.RawMessage `json:"expected"`
}

type Checker struct {
	ID        string      `json:"id"`
	Spec      CheckerSpec `json:"spec"`
	Hash      string      `json:"hash"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}

func RegisterChecker(ctx context.Context, scope experience.Scope, spec CheckerSpec, expiresAt time.Time) (Checker, error) {
	return transaction(ctx, scope, "checker_write", func(tx pgx.Tx, scope experience.Scope) (Checker, error) {
		out := Checker{ID: uuid.NewString(), Spec: spec, ExpiresAt: expiresAt}
		if err := validateChecker(spec); err != nil {
			return out, err
		}
		policy, err := policyTx(ctx, tx, scope)
		if err != nil {
			return out, err
		}
		if !expiresAt.After(time.Now()) || expiresAt.After(time.Now().Add(time.Duration(policy.Observation.RetentionSeconds)*time.Second)) {
			return out, experience.ErrInvalid
		}
		var expected any
		if err = decodeJSON(spec.Expected, &expected); err != nil {
			return out, experience.ErrInvalid
		}
		spec.Expected, _ = json.Marshal(expected)
		out.Spec, out.Hash = spec, jsonHash(spec)
		body, _ := json.Marshal(spec)
		err = tx.QueryRow(ctx, `INSERT INTO pangu.learning_checkers(id,namespace,project_id,content_hash,config_hash,actor_id,expires_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`, out.ID, scope.Namespace, scope.ProjectID, spec.ContentHash, out.Hash, scope.ActorID, expiresAt).Scan(&out.CreatedAt)
		if err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO pangu.learning_checker_bodies(checker_id,body) VALUES($1,$2)`, out.ID, body); err != nil {
			return out, err
		}
		return out, auditTx(ctx, tx, scope, out.ID, map[string]string{"config_hash": out.Hash})
	})
}

func ReadChecker(ctx context.Context, scope experience.Scope, id string) (Checker, error) {
	return transaction(ctx, scope, "checker_read", func(tx pgx.Tx, scope experience.Scope) (Checker, error) {
		return checkerTx(ctx, tx, scope, id)
	})
}

func checkerTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, id string) (Checker, error) {
	var out Checker
	var body []byte
	err := tx.QueryRow(ctx, `SELECT c.id,c.config_hash,c.created_at,c.expires_at,b.body FROM pangu.learning_checkers c
	 JOIN pangu.learning_checker_bodies b ON b.checker_id=c.id WHERE c.namespace=$1 AND c.project_id=$2 AND c.id=$3
	 AND c.expires_at>clock_timestamp() FOR SHARE OF c,b`, scope.Namespace, scope.ProjectID, id).
		Scan(&out.ID, &out.Hash, &out.CreatedAt, &out.ExpiresAt, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, experience.ErrInvalidated
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(body, &out.Spec); err != nil {
		return out, err
	}
	if jsonHash(out.Spec) != out.Hash {
		return out, experience.ErrInvalidated
	}
	return out, validateChecker(out.Spec)
}

func validateChecker(spec CheckerSpec) error {
	var expected any
	if spec.Kind != "json_assertion_v1" || !validHash(spec.ContentHash) || len(spec.Expected) > 256 ||
		decodeJSON(spec.Expected, &expected) != nil || !strings.HasPrefix(spec.JSONPointer, "/") || len(spec.JSONPointer) > 256 ||
		(spec.SourceKind != "report" && spec.SourceKind != "feedback") || (spec.SourceKind == "report" && spec.NodeID == "") ||
		(spec.SourceKind == "feedback" && spec.NodeID != "") || diagnostic.Sensitive(spec.Expected) {

		return experience.ErrInvalid
	}
	switch expected.(type) {
	case bool, json.Number, string:
	default:
		return experience.ErrInvalid
	}
	return nil
}
