package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// 初始化工作区前失败的已接受运行也需要保留期限；不触碰任何磁盘路径。
func expireUnprepared(ctx context.Context, conn *pgx.Conn, id string, finishedAt time.Time) error {
	var purged bool
	if err := conn.QueryRow(ctx, `SELECT s.purged_at IS NOT NULL FROM pangu.submissions s JOIN pangu.projects p ON p.id=s.project_id WHERE s.run_id=$1 AND p.worker_id=$2`, id, WorkerID()).Scan(&purged); err != nil {
		return err
	}
	if purged {
		return nil
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `WITH done AS (UPDATE pangu.submissions s SET finished_at=$2,purged_at=now()
 FROM pangu.projects p WHERE s.run_id=$1 AND s.project_id=p.id AND p.worker_id=$3 AND s.purged_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM pangu.node_attempts WHERE run_id=$1 AND (finished_at IS NULL OR finished_at>now()-interval '30 days'))
 RETURNING s.namespace,s.project_id,s.run_id,p.worker_id)
 INSERT INTO pangu.retention_records(namespace,project_id,run_id,worker_id,status) SELECT namespace,project_id,run_id,worker_id,'purged' FROM done`, id, finishedAt, WorkerID())
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("无法确认未准备运行的清理条件")
	}
	for _, query := range []string{
		`UPDATE pangu.node_attempts SET message='',truncated=true,evidence='[]'::jsonb,evidence_truncated=true WHERE run_id=$1`,
		`UPDATE pangu.schedules p SET active_slot=NULL FROM pangu.schedule_fires f,pangu.submissions s WHERE p.id=f.schedule_id AND p.active_slot=f.slot AND f.request_id=s.id AND s.run_id=$1`,
	} {
		if _, err := tx.Exec(ctx, query, id); err != nil {
			return err
		}
	}
	if err := purgePayloads(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
