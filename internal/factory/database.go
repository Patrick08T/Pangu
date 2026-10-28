package factory

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const sessionColumns = `id::text,project_id::text,coalesce(created_by::text,''),title,revision,updated_at`
const turnColumns = `id::text,coalesce(request_id::text,''),input,skill_digest,status,reply,error,result,created_at`

func Pending(ctx context.Context, ns string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT s.id::text FROM pangu.factory_sessions s WHERE s.namespace=$1 AND EXISTS(SELECT 1 FROM pangu.factory_turns t WHERE t.session_id=s.id AND t.status='pending') ORDER BY s.updated_at LIMIT 100`, ns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func authorize(ctx context.Context, tx pgx.Tx, ns, project, action string) error {
	if err := identity.Lock(ctx, tx, ns); err != nil {
		return err
	}
	if actor := identity.Current(ctx); actor.ID != "" {
		return identity.Authorize(ctx, actor, project, action, "")
	}
	return nil
}

func authorizeSession(ctx context.Context, s *Session) error {
	if actor := identity.Current(ctx); actor.ID != "" {
		return identity.Authorize(ctx, actor, s.ProjectID, "manage_factory", s.CreatedBy)
	}
	return nil
}

func Create(ctx context.Context, ns, project string) (*Session, error) {
	if _, err := uuid.Parse(project); err != nil {
		return nil, invalid("请选择项目")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = authorize(ctx, tx, ns, project, "configure"); err != nil {
		return nil, err
	}
	s, err := scanSession(tx.QueryRow(ctx, `INSERT INTO pangu.factory_sessions(id,namespace,project_id,created_by) SELECT $1,$2,id,nullif($4,'')::uuid FROM pangu.projects WHERE namespace=$2 AND id=$3 RETURNING `+sessionColumns, uuid.NewString(), ns, project, identity.Current(ctx).ID))
	if err != nil {
		return nil, err
	}
	return s, tx.Commit(ctx)
}

func Get(ctx context.Context, ns, id string) (*Session, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	s, err := scanSession(tx.QueryRow(ctx, "SELECT "+sessionColumns+" FROM pangu.factory_sessions WHERE namespace=$1 AND id=$2", ns, id))
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT "+turnColumns+" FROM pangu.factory_turns WHERE session_id=$1 ORDER BY sequence", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		t, err := scanTurn(rows)
		if err != nil {
			return nil, err
		}
		s.Turns = append(s.Turns, *t)
	}
	return s, rows.Err()
}

func List(ctx context.Context, ns, project string, offset int) ([]Session, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+sessionColumns+" FROM pangu.factory_sessions WHERE namespace=$1 AND project_id=$2 ORDER BY updated_at DESC,id LIMIT 100 OFFSET $3", ns, project, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func scanSession(row pgx.Row) (*Session, error) {
	s := &Session{Turns: []Turn{}}
	err := row.Scan(&s.ID, &s.ProjectID, &s.CreatedBy, &s.Title, &s.Revision, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, catalog.ErrNotFound
	}
	return s, err
}

func scanTurn(row pgx.Row) (*Turn, error) {
	t := &Turn{}
	var input, reply, result []byte
	err := row.Scan(&t.ID, &t.RequestID, &input, &t.SkillDigest, &t.Status, &reply, &t.Error, &result, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, catalog.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(input, &t.Input); err != nil {
		return nil, err
	}
	if len(reply) > 0 {
		if err = json.Unmarshal(reply, &t.Reply); err != nil {
			return nil, err
		}
	}
	if len(result) > 0 {
		if err = json.Unmarshal(result, &t.Result); err != nil {
			return nil, err
		}
	}
	return t, nil
}
