package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const experienceMaintenanceBatch = 8
const experienceMaintenanceItemBudget = 2 * time.Second
const experienceMemoryPurgeBatch = 32

// One short transaction per project per round prevents a large or corrupt project
// from starving its neighbours. The owning maintenance loop retains the cursor.
func purgeExperienceMemory(ctx context.Context, namespace string, cursor *string, preview bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if preview {
		return nil
	}
	if namespace == "" || cursor == nil {
		return experience.ErrInvalid
	}
	items, err := experienceMaintenanceProjects(ctx, namespace, *cursor)
	if err != nil {
		return err
	}
	var failures error
	for _, project := range items {
		if err := ctx.Err(); err != nil {
			return errors.Join(failures, err)
		}
		check, cancel := context.WithTimeout(ctx, experienceMaintenanceItemBudget)
		err := purgeExperienceProject(check, namespace, project)
		cancel()
		*cursor = project
		if err != nil {
			failures = errors.Join(failures, fmt.Errorf("memory retention project %s: %w", project, err))
		}
	}
	if len(items) < experienceMaintenanceBatch {
		*cursor = ""
	}
	return failures
}

func experienceMaintenanceProjects(ctx context.Context, namespace, after string) ([]string, error) {
	if after != "" {
		if _, err := uuid.Parse(after); err != nil {
			return nil, experience.ErrInvalid
		}
	}
	check, cancel := context.WithTimeout(ctx, experienceMaintenanceItemBudget)
	defer cancel()
	conn, err := database.Acquire(check)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	rows, err := conn.Query(check, `SELECT id::text FROM pangu.projects WHERE namespace=$1 AND ($2='' OR id>NULLIF($2,'')::uuid) ORDER BY id LIMIT $3`, namespace, after, experienceMaintenanceBatch)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func purgeExperienceProject(ctx context.Context, namespace, project string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = learning.PurgeExpiredTx(ctx, tx, namespace, project, experienceMemoryPurgeBatch); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
