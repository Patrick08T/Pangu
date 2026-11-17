package evolution

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

func transaction(ctx context.Context, fn func(pgx.Tx) error) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockScope(ctx context.Context, tx pgx.Tx, scope Scope) error {
	if scope.Namespace == "" || scope.ProjectID == "" {
		return ErrForbidden
	}
	if err := identity.Lock(ctx, tx, scope.Namespace); err != nil {
		return err
	}
	return experience.Lock(ctx, tx, scope.Namespace, scope.ProjectID)
}

func managerTx(ctx context.Context, tx pgx.Tx, scope Scope) error {
	var allowed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.users u JOIN pangu.projects p ON p.namespace=u.namespace
 WHERE u.id=$1 AND u.namespace=$2 AND p.id=$3 AND NOT u.disabled AND (u.is_admin OR EXISTS(
 SELECT 1 FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=p.id AND m.enabled AND m.role='manager')))`, scope.ActorID, scope.Namespace, scope.ProjectID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func policyTx(ctx context.Context, tx pgx.Tx, scope Scope, revision int64) (experience.Policy, error) {
	var p experience.Policy
	if err := lockScope(ctx, tx, scope); err != nil {
		return p, err
	}
	var data []byte
	var actor string
	var storedRevision int64
	err := tx.QueryRow(ctx, `SELECT policy,actor_id::text,revision FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, scope.Namespace, scope.ProjectID).Scan(&data, &actor, &storedRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrForbidden
	}
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if !p.Enabled || p.Revoked || p.ActorID != actor || p.ProjectID != scope.ProjectID || p.Revision != storedRevision || (revision > 0 && p.Revision != revision) {
		return p, ErrForbidden
	}
	if scope.ActorID != "" && scope.ActorID != actor {
		return p, ErrForbidden
	}
	scope.ActorID = actor
	return p, managerTx(ctx, tx, scope)
}

func jsonBytes(value any) []byte { data, _ := json.Marshal(value); return data }
