package schedule

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/database"
)

func RemotePending(ctx context.Context, namespace string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT id::text FROM pangu.schedules WHERE namespace=$1 AND NOT remote_synced ORDER BY remote_checked_at,created_at LIMIT 100", namespace)
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
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "UPDATE pangu.schedules SET remote_checked_at=now() WHERE namespace=$1 AND id=$2", namespace, id); err != nil {
		return nil, err
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "schedule:"+id); err != nil {
		return nil, err
	}
	plan, err := scanPlan(tx.QueryRow(ctx, "SELECT "+planColumns+" FROM pangu.schedules WHERE namespace=$1 AND id=$2", namespace, id))
	if err != nil || plan.Ready {
		return plan, err
	}
	remote, err := sync(ctx, plan)
	if err != nil {
		return plan, err
	}
	if _, err := tx.Exec(ctx, "UPDATE pangu.schedules SET remote_synced=true,hatchet_id=nullif($2,'')::uuid WHERE id=$1", id, remote); err != nil {
		return plan, err
	}
	if err := tx.Commit(ctx); err != nil {
		return plan, err
	}
	plan.Ready, plan.HatchetID = true, remote
	return plan, nil
}
