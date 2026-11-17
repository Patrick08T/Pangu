package experience

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Lock 必须在调用方的身份授权事务锁之后获取；所有经验变更使用相同顺序。
func Lock(ctx context.Context, tx pgx.Tx, namespace, projectID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "experience:"+namespace+":"+projectID)
	return err
}

func (s Store) authorizeTx(ctx context.Context, tx pgx.Tx, scope Scope) error {
	if s.Authorize == nil || tx == nil || scope.Namespace == "" || !s.allowsScope(scope) {
		return ErrForbidden
	}
	if _, err := uuid.Parse(scope.ProjectID); err != nil {
		return ErrForbidden
	}
	if err := s.Authorize(ctx, tx, scope); err != nil {
		return err
	}
	if err := Lock(ctx, tx, scope.Namespace, scope.ProjectID); err != nil {
		return err
	}
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.projects WHERE namespace=$1 AND id=$2)`, scope.Namespace, scope.ProjectID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrForbidden
	}
	return nil
}

func withStoreTx[T any](ctx context.Context, s Store, scope Scope, action string, fn func(pgx.Tx, Scope) (T, error)) (T, error) {
	var zero T
	if s.Authorize == nil {
		return zero, ErrForbidden
	}
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
	if err = s.authorizeTx(ctx, tx, scope); err != nil {
		return zero, err
	}
	value, err := fn(tx, scope)
	if err != nil {
		return zero, storageError(err)
	}
	if err = s.Authorize(ctx, tx, scope); err != nil {
		return zero, err
	}
	if err = tx.Commit(ctx); err != nil {
		return zero, storageError(err)
	}
	return value, nil
}

func storageError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return ErrConflict
	}
	return err
}

func scanJSON(row pgx.Row, target any) error {
	var data []byte
	if err := row.Scan(&data); err != nil {
		return storageError(err)
	}
	return json.Unmarshal(data, target)
}

func auditTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.experience_events(namespace,project_id,actor_id,action,subject_id,detail)
	 VALUES($1,$2,$3,$4,$5,$6)`, scope.Namespace, scope.ProjectID, scope.ActorID, scope.Action, id, data)
	return err
}
