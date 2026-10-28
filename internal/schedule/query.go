package schedule

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
)

func Fires(ctx context.Context, namespace, id string, offset int64) ([]Fire, error) {
	if offset < 0 {
		return nil, errors.New("offset 必须非负")
	}
	if _, err := Get(ctx, namespace, id); err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT schedule_id::text,slot,status,reason,coalesce(request_id::text,''),snapshot
		FROM pangu.schedule_fires WHERE schedule_id=$1 ORDER BY slot DESC OFFSET $2 LIMIT 100`, id, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Fire{}
	for rows.Next() {
		item, err := scanFire(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func Pending(ctx context.Context, namespace string) ([]Fire, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT f.schedule_id::text,f.slot,f.status,f.reason,coalesce(f.request_id::text,''),f.snapshot
		FROM pangu.schedule_fires f JOIN pangu.schedules p ON p.id=f.schedule_id
		WHERE p.namespace=$1 AND f.status='pending' ORDER BY f.last_checked_at,f.created_at LIMIT 100`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Fire{}
	for rows.Next() {
		item, err := scanFire(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func Checked(ctx context.Context, fire *Fire) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.schedule_fires SET last_checked_at=now() WHERE schedule_id=$1 AND slot=$2", fire.ScheduleID, fire.Slot)
	return err
}
