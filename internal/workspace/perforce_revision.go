package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func localRevision(ctx context.Context, conn *pgx.Conn, input workflow.NodeInput) (*workflow.Revision, bool, error) {
	s, err := readPerforce(ctx, conn, input.RunID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if s.LocalRoot == "" {
		return nil, false, nil
	}
	path, err := s.LocalDirectory(ctx)
	if err != nil {
		return nil, true, err
	}
	if !samePath(path, input.Workspace) {
		return nil, true, errors.New("本地 P4 执行目录与绑定不一致")
	}
	r := &workflow.Revision{LocalPerforce: true, Changelist: s.WorkChange, ChangedFiles: []string{}}
	var directory string
	err = conn.QueryRow(ctx, `SELECT w.base_commit,w.base_commit,a.before_commit,w.directory FROM pangu.workspace_runs w JOIN pangu.workspace_attempts a USING(run_id) WHERE w.run_id=$1 AND w.project_id=$2 AND w.worker_id=$3 AND w.ready AND w.expired_at IS NULL AND a.node_run_id=$4 AND a.directory=$5 AND a.status='running' AND a.before_commit<>'' ORDER BY a.attempt DESC LIMIT 1`, input.RunID, input.ProjectID, WorkerID(), input.NodeRunID, input.Workspace).Scan(&r.OriginCommit, &r.BaseCommit, &r.HeadCommit, &directory)
	if err != nil {
		return nil, true, errors.New("本地 P4 节点缺少执行前快照")
	}
	if input.Iteration != nil && input.Iteration.Gate {
		r.ReviewRepository = filepath.Join(directory, "base")
	}
	args := []string{"diff", "--no-ext-diff", "--name-only", "-z", r.OriginCommit, r.HeadCommit, "--"}
	err = streamLocalGit(ctx, filepath.Join(directory, "base"), args, func(name []byte) error {
		r.ChangedFiles = append(r.ChangedFiles, string(name))
		return nil
	})
	return r, true, err
}
