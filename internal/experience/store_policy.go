package experience

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s Store) Policy(ctx context.Context, scope Scope) (Policy, error) {
	return withStoreTx(ctx, s, scope, "policy_read", func(tx pgx.Tx, scope Scope) (Policy, error) {
		return policyTx(ctx, tx, scope)
	})
}

func policyTx(ctx context.Context, tx pgx.Tx, scope Scope) (Policy, error) {
	var policy Policy
	err := scanJSON(tx.QueryRow(ctx, `SELECT policy FROM pangu.experience_policies
	 WHERE namespace=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, scope.Namespace, scope.ProjectID), &policy)
	if errors.Is(err, ErrNotFound) {
		return Policy{ProjectID: scope.ProjectID}, nil
	}
	return policy, err
}

func (s Store) SavePolicy(ctx context.Context, scope Scope, policy Policy, expected int64) (Policy, error) {
	return withStoreTx(ctx, s, scope, "policy_write", func(tx pgx.Tx, scope Scope) (Policy, error) {
		if expected < 0 || (policy.ProjectID != "" && policy.ProjectID != scope.ProjectID) {
			return Policy{}, ErrInvalid
		}
		current, err := policyTx(ctx, tx, scope)
		if err != nil {
			return Policy{}, err
		}
		if current.Revision != expected {
			return Policy{}, ErrConflict
		}
		policy.ProjectID, policy.ActorID, policy.Revision = scope.ProjectID, scope.ActorID, expected+1
		if err = policy.Validate(); err != nil {
			return Policy{}, err
		}
		data, err := json.Marshal(policy)
		if err != nil {
			return Policy{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy)
		 VALUES($1,$2,$3,$4,$5)`, scope.Namespace, scope.ProjectID, policy.Revision, scope.ActorID, data)
		if err != nil {
			return Policy{}, err
		}
		if err = auditTx(ctx, tx, scope, scope.ProjectID, map[string]any{"revision": policy.Revision}); err != nil {
			return Policy{}, err
		}
		return policy, nil
	})
}
