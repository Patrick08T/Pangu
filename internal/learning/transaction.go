package learning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func transaction[T any](ctx context.Context, scope experience.Scope, action string, fn func(pgx.Tx, experience.Scope) (T, error)) (T, error) {
	var zero T
	conn, err := database.Open(ctx)
	if err != nil {
		return zero, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(context.Background())
	scope.Action = action
	if err = authorize(ctx, tx, scope); err != nil {
		return zero, err
	}
	if err = experience.Lock(ctx, tx, scope.Namespace, scope.ProjectID); err != nil {
		return zero, err
	}
	out, err := fn(tx, scope)
	if err != nil {
		return zero, err
	}
	if err = authorize(ctx, tx, scope); err != nil {
		return zero, err
	}
	if err = tx.Commit(ctx); err != nil {
		return zero, err
	}
	return out, nil
}

func digest(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

func jsonHash(value any) string {
	body, _ := json.Marshal(value)
	return digest(body)
}

func policyTx(ctx context.Context, tx pgx.Tx, scope experience.Scope) (experience.Policy, error) {
	policy, err := policyRecordTx(ctx, tx, scope)
	if err != nil {
		return policy, err
	}
	if !policy.Enabled {
		return policy, experience.ErrForbidden
	}
	return policy, nil
}

func sourcePolicyTx(ctx context.Context, tx pgx.Tx, scope experience.Scope) (experience.Policy, error) {
	policy, err := policyRecordTx(ctx, tx, scope)
	if err != nil {
		return policy, err
	}
	if !policy.ReadEnabled {
		return policy, experience.ErrForbidden
	}
	return policy, nil
}

func policyRecordTx(ctx context.Context, tx pgx.Tx, scope experience.Scope) (experience.Policy, error) {
	var policy experience.Policy
	var body []byte
	err := tx.QueryRow(ctx, `SELECT policy FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, scope.Namespace, scope.ProjectID).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return policy, experience.ErrForbidden
	}
	if err != nil {
		return policy, err
	}
	if err = json.Unmarshal(body, &policy); err != nil {
		return policy, err
	}
	if policy.Revoked || !policy.Observation.SourceRetentionAuthorized {
		return policy, experience.ErrForbidden
	}
	return policy, nil
}

func auditTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, id string, detail any) error {
	body, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.learning_events(namespace,project_id,actor_id,action,subject_id,detail)
	 VALUES($1,$2,$3,$4,$5,$6)`, scope.Namespace, scope.ProjectID, scope.ActorID, scope.Action, id, body)
	return err
}
