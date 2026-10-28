package audit

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type Entry struct {
	ID            string     `json:"id"`
	Namespace     string     `json:"-"`
	ActorID       string     `json:"actor_id"`
	TokenID       string     `json:"token_id,omitempty"`
	RequestID     string     `json:"request_id"`
	Action        string     `json:"action"`
	ObjectID      string     `json:"object_id,omitempty"`
	ProjectID     string     `json:"project_id,omitempty"`
	ResultID      string     `json:"result_id,omitempty"`
	ResultVersion *int       `json:"result_version,omitempty"`
	Outcome       string     `json:"outcome"`
	HTTPStatus    *int       `json:"http_status,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	TargetID      string     `json:"target_id,omitempty"`
	Role          string     `json:"role,omitempty"`
}

func Begin(ctx context.Context, e Entry) (*Entry, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	err = conn.QueryRow(ctx, `INSERT INTO pangu.audit_events(id,namespace,actor_id,token_id,request_id,action,object_id,target_id) VALUES($1,$2,$3,nullif($4,'')::uuid,$5,$6,nullif($7,'')::uuid,nullif($8,'')::uuid) RETURNING created_at`, e.ID, e.Namespace, e.ActorID, e.TokenID, e.RequestID, e.Action, e.ObjectID, e.TargetID).Scan(&e.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.Outcome = "pending"
	return &e, nil
}

func Finish(ctx context.Context, e *Entry, status int) error {
	outcome := "succeeded"
	if status >= 500 {
		outcome = "failed"
	} else if status >= 400 {
		outcome = "rejected"
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `UPDATE pangu.audit_events SET project_id=nullif($2,'')::uuid,result_id=nullif($3,'')::uuid,result_version=$4,outcome=$5,http_status=$6,finished_at=now(),target_id=nullif($8,'')::uuid,role=nullif($9,'') WHERE id=$1 AND namespace=$7 AND outcome='pending'`, e.ID, e.ProjectID, e.ResultID, e.ResultVersion, outcome, status, e.Namespace, e.TargetID, e.Role)
	if err == nil && result.RowsAffected() != 1 {
		return errors.New("审计记录已终结或不存在")
	}
	return err
}

func List(ctx context.Context, namespace, project string, offset int) ([]Entry, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT id::text,namespace,actor_id::text,coalesce(token_id::text,''),request_id::text,action,coalesce(object_id::text,''),coalesce(project_id::text,''),coalesce(result_id::text,''),result_version,
 CASE WHEN outcome='pending' AND created_at<now()-interval '1 minute' THEN 'unknown' ELSE outcome END,http_status,created_at,finished_at,coalesce(target_id::text,''),coalesce(role,'')
 FROM pangu.audit_events WHERE namespace=$1 AND ($2='' OR project_id=nullif($2,'')::uuid) ORDER BY created_at DESC,id DESC OFFSET $3 LIMIT 100`, namespace, project, offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Entry])
}

type entryKey struct{}

func WithEntry(ctx context.Context, e *Entry) context.Context {
	return context.WithValue(ctx, entryKey{}, e)
}
func Project(ctx context.Context, id string) {
	if e, ok := ctx.Value(entryKey{}).(*Entry); ok {
		e.ProjectID = id
	}
}

func Membership(ctx context.Context, user, role string) {
	if e, ok := ctx.Value(entryKey{}).(*Entry); ok {
		e.TargetID = user
		e.Role = role
	}
}
