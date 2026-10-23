package workspace

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
	"time"
)

type RetentionItem struct {
	RunID      string     `json:"run_id"`
	ProjectID  string     `json:"project_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiredAt  *time.Time `json:"expired_at,omitempty"`
	PurgedAt   *time.Time `json:"purged_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Status     string     `json:"status" db:"-"`
}

func RetentionCandidates(ctx context.Context, namespace string) ([]RetentionItem, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT run_id::text,project_id::text,created_at,expired_at,purged_at,finished_at FROM (
 SELECT r.run_id,r.project_id,r.created_at,r.expired_at,r.purged_at,r.finished_at,r.retention_checked_at FROM pangu.workspace_runs r
 JOIN pangu.projects p ON p.id=r.project_id WHERE p.namespace=$1 AND r.worker_id=$2 AND r.purged_at IS NULL
 UNION ALL SELECT s.run_id,s.project_id,s.created_at,NULL,s.purged_at,s.finished_at,s.retention_checked_at FROM pangu.submissions s
 JOIN pangu.projects p ON p.id=s.project_id WHERE s.namespace=$1 AND p.worker_id=$2 AND s.run_id IS NOT NULL AND s.purged_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM pangu.workspace_runs r WHERE r.run_id=s.run_id)
 ) candidates WHERE created_at<now()-interval '30 days' ORDER BY retention_checked_at,created_at,run_id LIMIT 100`, namespace, WorkerID())
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[RetentionItem])
	if items == nil && err == nil {
		items = []RetentionItem{}
	}
	return items, err
}

func RetentionChecked(ctx context.Context, id, status string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `WITH checked AS (UPDATE pangu.workspace_runs SET retention_checked_at=now() WHERE run_id=$1 RETURNING run_id,project_id),
 unprepared AS (UPDATE pangu.submissions SET retention_checked_at=now() WHERE run_id=$1 AND NOT EXISTS(SELECT 1 FROM pangu.workspace_runs WHERE run_id=$1) RETURNING run_id,project_id)
 INSERT INTO pangu.retention_records(namespace,project_id,run_id,worker_id,status)
 SELECT p.namespace,r.project_id,r.run_id,p.worker_id,$2 FROM (SELECT * FROM checked UNION ALL SELECT * FROM unprepared) r JOIN pangu.projects p ON p.id=r.project_id WHERE $2<>'purged'`, id, status)
	return err
}

// PruneHistory 只清理已完成产物清理的运行，保留配置对象、版本和活动计划。
func PruneHistory(ctx context.Context, namespace string) error {
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
	for _, query := range []string{
		`UPDATE pangu.schedule_fires f SET snapshot='{}'::jsonb FROM pangu.schedules s WHERE f.schedule_id=s.id AND s.namespace=$1 AND f.status IN ('skipped','failed') AND f.slot IS DISTINCT FROM s.active_slot AND f.created_at<now()-interval '30 days'`,
		`DELETE FROM pangu.schedule_fires f USING pangu.schedules s WHERE f.schedule_id=s.id AND s.namespace=$1 AND f.status<>'pending' AND f.slot IS DISTINCT FROM s.active_slot AND f.created_at<now()-interval '90 days'`,
		`DELETE FROM pangu.audit_events WHERE namespace=$1 AND created_at<now()-interval '180 days'`,
		`DELETE FROM pangu.retention_records WHERE namespace=$1 AND checked_at<now()-interval '180 days'`,
	} {
		if _, err = tx.Exec(ctx, query, namespace); err != nil {
			return err
		}
	}
	if err := pruneRuns(ctx, tx, namespace); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func pruneRuns(ctx context.Context, tx pgx.Tx, namespace string) error {
	rows, err := tx.Query(ctx, `SELECT r.run_id::text FROM (
 SELECT run_id,project_id,worker_id,finished_at FROM pangu.workspace_runs WHERE purged_at IS NOT NULL
 UNION ALL SELECT s.run_id,s.project_id,p.worker_id,s.finished_at FROM pangu.submissions s JOIN pangu.projects p ON p.id=s.project_id
 WHERE s.purged_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM pangu.workspace_runs r WHERE r.run_id=s.run_id)
 ) r JOIN pangu.projects p ON p.id=r.project_id WHERE p.namespace=$1 AND r.worker_id=$2 AND r.finished_at<now()-interval '90 days'
 AND NOT EXISTS(SELECT 1 FROM pangu.submissions s JOIN pangu.schedule_fires f ON f.request_id=s.id WHERE s.run_id=r.run_id)
 ORDER BY r.finished_at LIMIT 100`, namespace, WorkerID())
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(ids) == 0 {
		return err
	}
	for _, table := range []string{"submissions", "node_attempts", "run_cancellations", "workspace_runs"} {
		if _, err := tx.Exec(ctx, "DELETE FROM pangu."+table+" WHERE run_id=ANY($1::uuid[])", ids); err != nil {
			return err
		}
	}
	return nil
}
