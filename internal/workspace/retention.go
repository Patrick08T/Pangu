package workspace

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"time"
)

var ErrExpired = errors.New("运行产物、日志与重跑基准已过期")
var ErrLeased = errors.New("运行正在读取、下载或执行，稍后重试清理")

func Expired(ctx context.Context, id string) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	var expired bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.workspace_runs WHERE run_id=$1 AND expired_at IS NOT NULL) OR EXISTS(SELECT 1 FROM pangu.retention_records WHERE run_id=$1 AND status='purged')", id).Scan(&expired)
	return expired, err
}

// Lease 用数据库连接持有共享锁，进程退出后自动释放，不缓存配置。
func Lease(ctx context.Context, id string) (func(), error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	release := func() { _ = conn.Close(context.Background()) }
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock_shared(hashtextextended($1,0))", "retention:"+id); err != nil {
		release()
		return nil, err
	}
	var expired bool
	err = conn.QueryRow(ctx, "SELECT expired_at IS NOT NULL FROM pangu.workspace_runs WHERE run_id=$1", id).Scan(&expired)
	if expired {
		err = ErrExpired
	}
	if err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// Expire 由调度层确认整次运行终结满30天后调用；磁盘删除前持久记录到期意图。
func Expire(ctx context.Context, id string, finishedAt time.Time) error {
	before := time.Now().Add(-30 * 24 * time.Hour)
	if finishedAt.IsZero() || finishedAt.After(before) {
		return errors.New("运行终结尚未满30天")
	}
	conn, err := lockRun(ctx, id)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1,0))", "retention:"+id).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return ErrLeased
	}
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", id); err != nil {
		return err
	}
	record, err := readRun(ctx, conn, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return expireUnprepared(ctx, conn, id, finishedAt)
	}
	if err != nil {
		return err
	}
	root, err := workspaceRoot()
	if err != nil || record.WorkerID != WorkerID() || !samePath(record.Directory, filepath.Join(root, id)) {
		return errors.New("拒绝清理非当前 Worker 的托管目录")
	}
	result, err := conn.Exec(ctx, `UPDATE pangu.workspace_runs SET expired_at=coalesce(expired_at,now()),finished_at=coalesce(finished_at,$3) WHERE run_id=$1 AND created_at<$2
 AND NOT EXISTS(SELECT 1 FROM pangu.node_attempts WHERE run_id=$1 AND (finished_at IS NULL OR finished_at>=$2))`, id, before, finishedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("运行仍活跃或保留期未到")
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err = directory.RemoveAll(id); err != nil {
		return err
	}
	return clearExpiredData(ctx, conn, id)
}

func clearExpiredData(ctx context.Context, conn *pgx.Conn, id string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	for _, query := range []string{
		`UPDATE pangu.schedules p SET active_slot=NULL FROM pangu.schedule_fires f,pangu.submissions s WHERE p.id=f.schedule_id AND p.active_slot=f.slot AND f.request_id=s.id AND s.run_id=$1`,
		"DELETE FROM pangu.workspace_checkpoints WHERE run_id=$1",
		"DELETE FROM pangu.workspace_attempts WHERE run_id=$1",
		"DELETE FROM pangu.research_reports WHERE run_id=$1",
		"UPDATE pangu.node_attempts SET message='',truncated=true,evidence='[]'::jsonb,evidence_truncated=true WHERE run_id=$1",
		`WITH done AS (UPDATE pangu.workspace_runs SET purged_at=now() WHERE run_id=$1 AND purged_at IS NULL RETURNING *)
 INSERT INTO pangu.retention_records(namespace,project_id,run_id,worker_id,status) SELECT p.namespace,d.project_id,d.run_id,d.worker_id,'purged' FROM done d JOIN pangu.projects p ON p.id=d.project_id`,
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

func purgePayloads(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, "SELECT pangu.purge_run_payloads($1)", id)
	return err
}
