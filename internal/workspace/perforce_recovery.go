package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/jackc/pgx/v5"
)

// 调用方已持有本地工作区锁，并确认旧进程停止；先补齐中断尝试的快照与文件归属。
func recoverLocalAttempts(ctx context.Context, conn *pgx.Conn, current *perforce.Snapshot) (result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return err
	}
	defer func() { result = finish(result) }()
	ctx, cancel := context.WithTimeout(ctx, localSnapshotTimeout)
	defer cancel()
	rows, err := conn.Query(ctx, `SELECT a.run_id,a.node_run_id,a.attempt,a.before_commit FROM pangu.workspace_attempts a JOIN pangu.perforce_sources s USING(run_id) JOIN pangu.workspace_runs w USING(run_id) WHERE w.worker_id=$1 AND s.local_path=$2 AND s.config->>'port'=$3 AND s.client=$4 AND a.before_commit<>'' AND NOT a.p4_tracked ORDER BY a.created_at`, WorkerID(), current.LocalRoot, current.Config.Port, current.Client)
	if err != nil {
		return err
	}
	type pending struct {
		run, node string
		attempt   int
		before    string
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (pending, error) {
		var p pending
		err := row.Scan(&p.run, &p.node, &p.attempt, &p.before)
		return p, err
	})
	if err != nil {
		return err
	}
	for _, p := range items {
		record, err := readRun(ctx, conn, p.run)
		if err != nil {
			return err
		}
		root, err := workspaceRoot()
		if err != nil || record.Directory != filepath.Join(root, record.ID) || record.ExpiredAt != nil {
			return errors.New("中断的 P4 运行快照不可用，请先恢复快照目录")
		}
		s, err := readPerforce(ctx, conn, p.run)
		if err != nil {
			return err
		}
		if _, err = finishLocalAttempt(ctx, conn, record, s, p.node, p.attempt, p.before, false); err != nil {
			return err
		}
	}
	return nil
}
