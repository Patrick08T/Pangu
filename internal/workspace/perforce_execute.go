package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

// 同一 Worker 的本地 P4 操作串行，覆盖 Client 别名和相互重叠的映射。
func lockLocalWorkspace(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", "perforce-local:"+WorkerID())
	return err
}

func executeLocalPerforce(ctx context.Context, conn *pgx.Conn, record *Run, s *perforce.Snapshot, node string, attempt int, readOnly bool, execute func(string) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	if err := lockLocalWorkspace(ctx, conn); err != nil {
		return workflow.NodeOutput{}, err
	}
	if err := checkLocalProcesses(ctx, conn); err != nil {
		return workflow.NodeOutput{}, err
	}
	path, err := s.LocalDirectory(ctx)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	if err = recoverLocalAttempts(ctx, conn, s); err != nil {
		return workflow.NodeOutput{}, err
	}
	s, err = readPerforce(ctx, conn, record.ID)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	before, err := localSnapshot(ctx, s, filepath.Join(record.Directory, "base"), record.HeadCommit)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	if !readOnly {
		if err = ensureLocalChange(ctx, conn, s); err != nil {
			return workflow.NodeOutput{}, err
		}
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_attempts(run_id,node_run_id,attempt,directory,base_commit,before_commit,status) VALUES($1,$2,$3,$4,$5,$6,'running')`, record.ID, node, attempt, path, record.HeadCommit, before)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	defer failAttempt(node, attempt)
	out, runErr := execute(path)
	if errors.Is(runErr, workflow.ErrStopUnconfirmed) {
		return out, runErr
	}
	// 失败或取消也保留现场快照和 P4 归属，不对用户目录回退。
	finish, cancel := context.WithTimeout(context.Background(), localSnapshotTimeout)
	defer cancel()
	after, saveErr := finishLocalAttempt(finish, conn, record, s, node, attempt, before, readOnly)
	if err = errors.Join(runErr, ctx.Err(), saveErr); err != nil {
		return out, err
	}
	return out, publish(ctx, conn, record, node, after, attempt, out)
}

func checkLocalProcesses(ctx context.Context, conn *pgx.Conn) error {
	if err := checkLocalGitOperation(ctx); err != nil {
		return err
	}
	var active bool
	err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.workspace_attempts a JOIN pangu.perforce_sources s USING(run_id) JOIN pangu.node_attempts n ON n.node_run_id=a.node_run_id AND n.attempt=a.attempt JOIN pangu.workspace_runs w ON w.run_id=a.run_id WHERE s.local_path<>'' AND w.worker_id=$1 AND n.finished_at IS NULL AND NOT a.p4_tracked)`, WorkerID()).Scan(&active)
	if err != nil {
		return err
	}
	if active {
		return errors.New("先前的本地 P4 执行尚未确认停止，暂不启动新的工作区操作")
	}
	return nil
}

func finishLocalAttempt(ctx context.Context, conn *pgx.Conn, record *Run, s *perforce.Snapshot, node string, attempt int, before string, readOnly bool) (_ string, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() { result = finish(result) }()
	ctx, cancel := context.WithTimeout(ctx, localSnapshotTimeout)
	defer cancel()
	after, err := localSnapshot(ctx, s, filepath.Join(record.Directory, "base"), before)
	if err != nil {
		return "", err
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.workspace_attempts SET after_commit=$3 WHERE node_run_id=$1 AND attempt=$2", node, attempt, after)
	if err != nil {
		return "", err
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.perforce_sources SET local_commit=$2 WHERE run_id=$1", record.ID, after)
	if err != nil {
		return "", err
	}
	paths, err := localChangedPaths(ctx, filepath.Join(record.Directory, "base"), before, after)
	if err != nil {
		return after, err
	}
	if len(paths) > 0 {
		if err = ensureLocalChange(ctx, conn, s); err != nil {
			return after, err
		}
	}
	other, err := s.TrackLocal(ctx, paths)
	if err != nil {
		return after, err
	}
	_, err = conn.Exec(ctx, `UPDATE pangu.perforce_sources SET existing_changes=(SELECT coalesce(jsonb_agg(DISTINCT value),'[]') FROM jsonb_array_elements(existing_changes || $2::jsonb)),tracked_paths=(SELECT coalesce(jsonb_agg(DISTINCT value),'[]') FROM jsonb_array_elements(tracked_paths || $3::jsonb)) WHERE run_id=$1`, record.ID, other, paths)
	if err == nil {
		_, err = conn.Exec(ctx, "UPDATE pangu.workspace_attempts SET p4_tracked=true WHERE node_run_id=$1 AND attempt=$2", node, attempt)
	}
	if err == nil && readOnly && len(paths) > 0 {
		return after, &workflow.Failure{Kind: "permission", Cause: errors.New("只读节点修改了本地 P4 文件；现场和 changelist 已保留，拒绝发布成功检查点")}
	}
	return after, err
}

func ensureLocalChange(ctx context.Context, conn *pgx.Conn, s *perforce.Snapshot) error {
	if s.WorkChange > 0 {
		return nil
	}
	var err error
	s.WorkChange, err = s.PendingChange(ctx)
	if err == nil {
		_, err = conn.Exec(ctx, "UPDATE pangu.perforce_sources SET work_change=$2 WHERE run_id=$1", s.RunID, s.WorkChange)
	}
	return err
}
