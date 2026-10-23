package schedule

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/identity"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("定时计划不存在或不属于当前命名空间")
var ErrConflict = errors.New("计划名称已用于其他配置，或计划修订已变化")

type Plan struct {
	CreatedBy  string              `json:"created_by,omitempty"`
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Expression string              `json:"expression"`
	Workflow   workflow.Definition `json:"workflow"`
	Revision   int                 `json:"revision"`
	Paused     bool                `json:"paused"`
	Deleted    bool                `json:"deleted"`
	Ready      bool                `json:"ready"`
	CreatedAt  time.Time           `json:"created_at"`
	HatchetID  string              `json:"-"`
	ActiveSlot *time.Time          `json:"-"`
}

const planColumns = `id::text,name,expression,workflow,revision,paused,deleted,created_at,coalesce(hatchet_id::text,''),active_slot,remote_synced,coalesce(created_by::text,'')`

func Create(ctx context.Context, namespace string, c workflow.Cron) (*Plan, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(c.Workflow.ProjectID); err != nil {
		return nil, errors.New("定时计划必须绑定项目")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := transaction(ctx, conn, namespace)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = authorizeChange(ctx, c.Workflow.ProjectID, "schedule", ""); err != nil {
		return nil, err
	}
	data, err := json.Marshal(c.Workflow)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	_, err = tx.Exec(ctx, `INSERT INTO pangu.schedules(id,namespace,project_id,name,expression,workflow,created_by)
		VALUES($1,$2,$3,$4,$5,$6,nullif($7,'')::uuid) ON CONFLICT(namespace,name) DO NOTHING`, id, namespace, c.Workflow.ProjectID, c.Name, c.Expression, data, identity.Current(ctx).ID)
	if err != nil {
		return nil, err
	}
	item, err := scanPlan(tx.QueryRow(ctx, "SELECT "+planColumns+" FROM pangu.schedules WHERE namespace=$1 AND name=$2", namespace, c.Name))
	if err != nil {
		return nil, err
	}
	previous, err := json.Marshal(item.Workflow)
	if err != nil {
		return nil, err
	}
	if item.CreatedBy != identity.Current(ctx).ID || item.Deleted || item.Expression != c.Expression || !bytes.Equal(data, previous) {
		return nil, ErrConflict
	}
	return item, tx.Commit(ctx)
}

func Get(ctx context.Context, namespace, id string) (*Plan, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scanPlan(conn.QueryRow(ctx, "SELECT "+planColumns+" FROM pangu.schedules WHERE namespace=$1 AND id=$2", namespace, id))
}

func scanPlan(row pgx.Row) (*Plan, error) {
	out := &Plan{}
	var data []byte
	err := row.Scan(&out.ID, &out.Name, &out.Expression, &data, &out.Revision, &out.Paused, &out.Deleted, &out.CreatedAt, &out.HatchetID, &out.ActiveSlot, &out.Ready, &out.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, json.Unmarshal(data, &out.Workflow)
}

func List(ctx context.Context, namespace string, offset int64) ([]Plan, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+planColumns+" FROM pangu.schedules WHERE namespace=$1 AND NOT deleted ORDER BY created_at,id OFFSET $2 LIMIT 100", namespace, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Plan{}
	for rows.Next() {
		item, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func Registered(ctx context.Context, id, hatchetID string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.schedules SET hatchet_id=$2,remote_synced=true WHERE id=$1", id, hatchetID)
	return err
}

func Delete(ctx context.Context, namespace, id string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := transaction(ctx, conn, namespace)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "schedule:"+id); err != nil {
		return err
	}
	plan, err := scanPlan(tx.QueryRow(ctx, "SELECT "+planColumns+" FROM pangu.schedules WHERE namespace=$1 AND id=$2", namespace, id))
	if err != nil {
		return err
	}
	if err = authorizeChange(ctx, plan.Workflow.ProjectID, "manage_schedule", plan.CreatedBy); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, "UPDATE pangu.schedules SET deleted=true,paused=true,remote_synced=false WHERE id=$1 AND namespace=$2", id, namespace)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
