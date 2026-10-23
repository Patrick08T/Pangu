package schedule

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func Save(ctx context.Context, namespace, id string, revision int, c workflow.Cron, paused bool) (*Plan, error) {
	if err := c.Validate(); err != nil {
		return nil, err
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
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "schedule:"+id); err != nil {
		return nil, err
	}
	plan, err := scanPlan(tx.QueryRow(ctx, "SELECT "+planColumns+" FROM pangu.schedules WHERE namespace=$1 AND id=$2", namespace, id))
	if err != nil {
		return nil, err
	}
	if err = authorizeChange(ctx, plan.Workflow.ProjectID, "manage_schedule", plan.CreatedBy); err != nil {
		return nil, err
	}
	if plan.Deleted {
		return nil, ErrNotFound
	}
	if c.Workflow.ProjectID != plan.Workflow.ProjectID {
		return nil, errors.New("定时计划不能迁移到其他项目")
	}
	data, err := json.Marshal(c.Workflow)
	if err != nil {
		return nil, err
	}
	old, err := json.Marshal(plan.Workflow)
	if err != nil {
		return nil, err
	}
	if plan.Revision == revision+1 && plan.Name == c.Name && plan.Expression == c.Expression && plan.Paused == paused && bytes.Equal(data, old) {
		return plan, nil
	}
	if plan.Revision != revision {
		return nil, ErrConflict
	}
	plan, err = scanPlan(tx.QueryRow(ctx, `UPDATE pangu.schedules SET name=$2,expression=$3,workflow=$4,paused=$5,revision=revision+1,remote_synced=false WHERE id=$1 RETURNING `+planColumns, id, c.Name, c.Expression, data, paused))
	if err != nil {
		return nil, err
	}
	return plan, tx.Commit(ctx)
}
