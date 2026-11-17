package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/jackc/pgx/v5"
)

func deliverLocalPerforce(ctx context.Context, conn *pgx.Conn, record *Run, s *perforce.Snapshot) (_ *P4Delivery, result error) {
	ctx, finish, err := beginLocalPerforceOperation(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer func() { result = finish(result) }()
	if err := checkLocalProcesses(ctx, conn); err != nil {
		return nil, err
	}
	root, rootErr := workspaceRoot()
	if rootErr != nil || record.Directory != filepath.Join(root, record.ID) || !record.Ready || record.WorkerID != WorkerID() {
		return nil, errors.New("本地 P4 工作区属于其他 Worker 或尚未就绪")
	}
	if err := recoverLocalAttempts(ctx, conn, s); err != nil {
		return nil, err
	}
	updated, readErr := readPerforce(ctx, conn, record.ID)
	if readErr != nil {
		return nil, readErr
	}
	*s = *updated
	_, err = conn.Exec(ctx, `INSERT INTO pangu.perforce_deliveries(run_id,checkpoint,client,changelist,status) VALUES($1,$2,$3,nullif($4,0),'preparing') ON CONFLICT(run_id) DO UPDATE SET checkpoint=CASE WHEN pangu.perforce_deliveries.checkpoint='' THEN EXCLUDED.checkpoint ELSE pangu.perforce_deliveries.checkpoint END`, record.ID, s.LocalCommit, s.Client, s.WorkChange)
	if err != nil {
		return nil, err
	}
	current, err := readP4Delivery(ctx, conn, record.ID)
	if err != nil {
		return nil, err
	}
	if current.Status == "shelved" || current.Status == "empty" {
		return current, nil
	}
	if current.Checkpoint != s.LocalCommit {
		return nil, errors.New("交付快照已固定，后续修改需要新的运行")
	}
	var allowed []string
	if err = conn.QueryRow(ctx, "SELECT tracked_paths FROM pangu.perforce_sources WHERE run_id=$1", record.ID).Scan(&allowed); err != nil {
		return nil, err
	}
	shelved, err := shelveLocalCheckpoint(ctx, record, s, allowed)
	status, message := "empty", ""
	if shelved {
		status = "shelved"
	}
	if err != nil {
		status = "failed"
		message, _ = diagnostic.Text(err.Error(), 4096)
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.perforce_deliveries SET status=$2,error=$3,changelist=nullif($4,0),updated_at=now() WHERE run_id=$1", record.ID, status, message, s.WorkChange)
	if err != nil {
		return nil, err
	}
	return readP4Delivery(ctx, conn, record.ID)
}

func shelveLocalCheckpoint(ctx context.Context, record *Run, s *perforce.Snapshot, allowed []string) (bool, error) {
	base := filepath.Join(record.Directory, "base")
	current, err := localSnapshot(ctx, s, base, s.LocalCommit)
	if err != nil {
		return false, err
	}
	equal, err := sameLocalTrees(ctx, base, s.LocalCommit, current)
	if err != nil {
		return false, err
	}
	if !equal {
		return false, errors.New("本地工作区在运行后已变化，拒绝把当前内容作为旧运行成果 shelve；请发起新运行或在 P4V 审查")
	}
	return s.ShelveLocal(ctx, allowed)
}
