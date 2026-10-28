package webhook

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/jackc/pgx/v5"
)

func Receipt(ctx context.Context, namespace, id, eventID, digest string) (*submission.Record, error) {
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
	if !allowed || !hook.Enabled {
		return nil, ErrDisabled
	}
	var request, hash string
	err = tx.QueryRow(ctx, `SELECT e.request_id::text,e.payload_hash FROM pangu.github_events e JOIN pangu.github_hooks h ON h.id=e.hook_id WHERE h.namespace=$1 AND h.id=$2 AND (e.delivery_id=$3 OR e.payload_hash=$4) ORDER BY (e.delivery_id=$3) DESC LIMIT 1`, namespace, id, eventID, digest).Scan(&request, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if hash != digest {
		return nil, ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.github_events(hook_id,delivery_id,payload_hash,request_id) VALUES($1,$2,$3,$4) ON CONFLICT(hook_id,delivery_id) DO NOTHING`, id, eventID, digest, request)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return submission.Get(ctx, namespace, request)
}
