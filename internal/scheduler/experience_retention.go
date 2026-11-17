package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/jackc/pgx/v5"
)

const experienceRetentionPageSize = 64

type experienceRetentionCursor struct {
	at      time.Time
	request string
}

// ReconcileExperienceRetention is mounted by the scheduler owner, not by this module.
// It never dispatches/replays a case and never infers remote completion from absent rows.
func (b Backend) ReconcileExperienceRetention(ctx context.Context) error {
	return reconcileExperienceRetention(ctx, b.Namespace())
}

func reconcileExperienceRetention(ctx context.Context, namespace string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var high experienceRetentionCursor
	err = conn.QueryRow(ctx, `SELECT created_at,request_id::text FROM pangu.evolution_case_plans
	 WHERE namespace=$1 AND erased_at IS NULL ORDER BY created_at DESC,request_id DESC LIMIT 1`, namespace).Scan(&high.at, &high.request)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var after *experienceRetentionCursor
	for {
		page, err := experienceRetentionPage(ctx, conn, namespace, after, high)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		for _, item := range page {
			if err := experienceRetainCase(ctx, namespace, item.request); err != nil {
				return err
			}
		}
		after = &page[len(page)-1]
	}
}

// A fixed upper key bounds the traversal; neither deletes nor busy early cases
// shift later rows as OFFSET/LIMIT would. No private plan bodies are loaded.
func experienceRetentionPage(ctx context.Context, conn *pgx.Conn, namespace string, after *experienceRetentionCursor, high experienceRetentionCursor) ([]experienceRetentionCursor, error) {
	var at any
	key := "00000000-0000-0000-0000-000000000000"
	if after != nil {
		at, key = after.at, after.request
	}
	rows, err := conn.Query(ctx, `SELECT created_at,request_id::text FROM pangu.evolution_case_plans
	 WHERE namespace=$1 AND erased_at IS NULL AND ($2::timestamptz IS NULL OR (created_at,request_id)>($2,$3::uuid))
	 AND (created_at,request_id)<=($4,$5::uuid) ORDER BY created_at,request_id LIMIT $6`, namespace, at, key, high.at, high.request, experienceRetentionPageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var page []experienceRetentionCursor
	for rows.Next() {
		var item experienceRetentionCursor
		if err := rows.Scan(&item.at, &item.request); err != nil {
			return nil, err
		}
		page = append(page, item)
	}
	return page, rows.Err()
}

func experienceRetainCase(ctx context.Context, namespace, request string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var locked bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "evolution-case:"+request).Scan(&locked)
	if err != nil || !locked {
		return err
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	project, run, ready, err := experienceRetentionStopped(ctx, tx, namespace, request)
	if err != nil || !ready {
		return err
	}
	due, err := experienceRetentionDue(ctx, tx, namespace, project, request)
	if err != nil || !due {
		return err
	}
	if err := workspace.PurgeExperienceRunTx(ctx, tx, namespace, project, request, run); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func experienceRetentionStopped(ctx context.Context, tx pgx.Tx, namespace, request string) (string, string, bool, error) {
	var project, run string
	var ready bool
	err := tx.QueryRow(ctx, `SELECT p.project_id::text,r.run_id::text,
	 r.stop_confirmed AND r.status<>'running' AND r.finished_at IS NOT NULL
	 AND NOT EXISTS(SELECT 1 FROM pangu.node_attempts a WHERE a.finished_at IS NULL AND
	 (a.run_id=r.run_id OR a.node_run_id IN (SELECT n.node_run_id FROM pangu.evolution_node_runs n WHERE n.request_id=p.request_id)))
	 AND NOT EXISTS(SELECT 1 FROM pangu.workspace_runs w WHERE w.run_id=r.run_id)
	 FROM pangu.evolution_case_plans p JOIN pangu.evolution_case_runtime r USING(request_id)
	 JOIN pangu.submissions s ON s.id=p.request_id AND s.namespace=p.namespace AND s.project_id=p.project_id AND s.run_id=r.run_id
	 WHERE p.namespace=$1 AND p.request_id=$2 AND p.erased_at IS NULL FOR UPDATE OF p,r,s`, namespace, request).Scan(&project, &run, &ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	return project, run, ready, err
}
