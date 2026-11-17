package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func RevisionFor(ctx context.Context, input workflow.NodeInput) (*workflow.Revision, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	if local, handled, err := localRevision(ctx, conn, input); handled || err != nil {
		return local, err
	}
	var revision workflow.Revision
	var directory string
	err = conn.QueryRow(ctx, `SELECT coalesce(s.snapshot#>>'{fork,origin_commit}',w.base_commit),w.base_commit,w.head_commit,w.directory
 FROM pangu.workspace_runs w JOIN pangu.workspace_attempts a ON a.run_id=w.run_id
 LEFT JOIN pangu.submissions s ON s.run_id=w.run_id
 WHERE w.run_id=$1 AND w.project_id=$2 AND w.worker_id=$3 AND w.ready AND w.expired_at IS NULL
 AND a.node_run_id=$4 AND a.directory=$5 AND a.status='running' AND a.base_commit=w.head_commit
 AND (w.head_commit=w.base_commit OR EXISTS(SELECT 1 FROM pangu.workspace_checkpoints c WHERE c.run_id=w.run_id AND c.commit_id=w.head_commit))`,
		input.RunID, input.ProjectID, WorkerID(), input.NodeRunID, input.Workspace).Scan(&revision.OriginCommit, &revision.BaseCommit, &revision.HeadCommit, &directory)
	if err != nil {
		return nil, errors.New("当前工作区不属于本次运行的有效检查点")
	}
	head, err := git(ctx, input.Workspace, "rev-parse", "HEAD")
	if err != nil || head != revision.HeadCommit {
		return nil, errors.New("工作区 HEAD 与平台检查点不一致")
	}
	status, err := git(ctx, input.Workspace, "status", "--porcelain=v1")
	if err != nil || status != "" {
		return nil, errors.New("节点开始前工作区含未发布变化")
	}
	base := filepath.Join(directory, "base")
	if _, err = git(ctx, base, "merge-base", "--is-ancestor", revision.OriginCommit, revision.HeadCommit); err != nil {
		return nil, errors.New("当前检查点不在起始基线的提交链上")
	}
	diff, err := git(ctx, base, "diff", "--no-ext-diff", "--name-only", "-z", revision.OriginCommit, revision.HeadCommit, "--")
	if err != nil {
		return nil, err
	}
	revision.ChangedFiles = []string{}
	if diff != "" {
		revision.ChangedFiles = strings.Split(strings.TrimSuffix(diff, "\x00"), "\x00")
	}
	return &revision, nil
}

// ContentRevision 比较文件树而不是提交号；只读节点也会生成新的检查点提交。
func ContentRevision(ctx context.Context, run, commit string) (string, error) {
	if !commitID.MatchString(commit) {
		return "", errors.New("代码检查点标识无效")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	record, err := readRun(ctx, conn, run)
	if err != nil {
		return "", err
	}
	if record.WorkerID != WorkerID() || record.ExpiredAt != nil {
		return "", errors.New("代码检查点不属于当前 Worker")
	}
	local, err := readPerforce(ctx, conn, run)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if local != nil && local.LocalRoot != "" {
		return runLocalGit(ctx, filepath.Join(record.Directory, "base"), "rev-parse", "--verify", commit+"^{tree}")
	}
	return git(ctx, filepath.Join(record.Directory, "base"), "rev-parse", "--verify", commit+"^{tree}")
}
