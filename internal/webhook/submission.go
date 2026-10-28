package webhook

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func Accept(ctx context.Context, namespace, id, eventID, digest string, d workflow.Definition) (*submission.Record, error) {
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
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return nil, err
	}
	hook, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.github_hooks WHERE namespace=$1 AND id=$2 FOR UPDATE", namespace, id))
	if err != nil {
		return nil, err
	}
	allowed, err := identity.ScheduleOwner(ctx, tx, hook.ActorID, hook.ProjectID)
	if err != nil {
		return nil, err
	}
	if !hook.Enabled || !allowed {
		return nil, ErrDisabled
	}
	if d.VersionID != hook.VersionID || d.ProjectID != hook.ProjectID {
		return nil, ErrInvalid
	}
	var old string
	err = tx.QueryRow(ctx, "SELECT payload_hash FROM pangu.github_events WHERE hook_id=$1 AND delivery_id=$2", id, eventID).Scan(&old)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil && old != digest {
		return nil, ErrConflict
	}
	ctx = identity.WithActor(ctx, identity.Actor{ID: hook.ActorID, Namespace: namespace})
	item, err := submission.PrepareTx(ctx, tx, namespace, "github", id+":"+digest, submission.Snapshot{Workflow: d})
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.github_events(hook_id,delivery_id,payload_hash,request_id) VALUES($1,$2,$3,$4) ON CONFLICT(hook_id,delivery_id) DO NOTHING`, id, eventID, digest, item.ID)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

type Event struct {
	DeliveryID string `json:"delivery_id"`
	RequestID  string `json:"request_id"`
	RunID      string `json:"run_id"`
	CreatedAt  string `json:"created_at"`
}

func Events(ctx context.Context, namespace, id string, offset int) ([]Event, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT e.delivery_id::text,e.request_id::text,coalesce(s.run_id::text,''),to_char(e.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
 FROM pangu.github_events e JOIN pangu.github_hooks h ON h.id=e.hook_id JOIN pangu.submissions s ON s.id=e.request_id WHERE h.namespace=$1 AND h.id=$2 ORDER BY e.created_at DESC,e.delivery_id DESC LIMIT 100 OFFSET $3`, namespace, id, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Event])
}
