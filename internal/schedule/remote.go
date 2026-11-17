package schedule

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func RemotePending(ctx context.Context, namespace string) ([]string, error) {
	conn, err := database.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	rows, err := conn.Query(ctx, `SELECT id::text FROM pangu.schedules s WHERE namespace=$1 AND NOT remote_synced AND NOT `+remoteEvolutionOwned+` ORDER BY remote_checked_at,created_at LIMIT 100`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func ReconcileRemote(ctx context.Context, namespace, id string, sync func(context.Context, *Plan) (string, error)) (*Plan, error) {
	target, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrNotFound
	}
	id = target.String() // UUID aliases must not create distinct advisory mutexes.
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background()) // Releases the session lock on every exit.
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, "schedule-remote:"+id); err != nil {
		return nil, err
	}
	plan, snapshot, err := remoteEvolutionPrepare(ctx, conn, namespace, id)
	if err != nil || plan.Ready {
		return plan, err
	}
	// The shared mutex spans RPC, but no SQL transaction or row lock does.
	remote, err := sync(ctx, plan)
	if err != nil {
		return plan, err
	}
	return remoteEvolutionReceipt(ctx, conn, namespace, id, snapshot, remote)
}
