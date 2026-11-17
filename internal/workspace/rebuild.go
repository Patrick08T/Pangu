package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EnsureRebuild 新建原始干净基线工作区；取消、停机确认与原申请人授权由调度层负责。
// 恢复调用仅校验已有工作区，不重置其新成果，也不继承来源成果或审批。
func EnsureRebuild(ctx context.Context, runID, projectID, namespace, sourceRunID string) error {
	if runID == sourceRunID || sourceRunID == "" {
		return errors.New("干净重建必须使用独立的新运行")
	}
	return withRebuildSource(ctx, sourceRunID, projectID, namespace, func(project *Project, source *Run, origin string) error {
		return ensureRebuildRun(ctx, runID, project, source, origin)
	})
}

func rebuildOrigin(ctx context.Context, source, project, namespace string) (string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	seen := map[string]bool{}
	for !seen[source] {
		seen[source] = true
		var base, parent, origin string
		var fork bool
		err = conn.QueryRow(ctx, `SELECT w.base_commit,coalesce(s.snapshot->>'source_run_id',''),
 coalesce(s.snapshot#>>'{fork,origin_commit}',''),coalesce(s.snapshot->'fork'<>'null'::jsonb,false)
 FROM pangu.workspace_runs w JOIN pangu.submissions s ON s.run_id=w.run_id AND s.project_id=w.project_id
 JOIN pangu.projects p ON p.id=w.project_id AND p.namespace=s.namespace
 WHERE w.run_id=$1 AND w.project_id=$2 AND s.namespace=$3 AND s.purged_at IS NULL
 AND s.snapshot#>>'{workflow,project_id}'=w.project_id::text`, source, project, namespace).Scan(&base, &parent, &origin, &fork)
		if err != nil {
			return "", errors.New("来源提交身份或原始基线证据不可用")
		}
		if fork {
			return origin, nil
		}
		if parent == "" {
			return base, nil
		}
		if _, err = uuid.Parse(parent); err != nil {
			return "", err
		}
		source = parent
	}
	return "", errors.New("来源提交链存在循环，无法证明干净基线")
}

func ensureRebuildRun(ctx context.Context, id string, project *Project, source *Run, origin string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}
	conn, err := lockRun(ctx, id)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if expired, err := Expired(ctx, id); err != nil || expired {
		if err != nil {
			return err
		}
		return ErrExpired
	}
	record, err := readRun(ctx, conn, id)
	if errors.Is(err, pgx.ErrNoRows) {
		record = &Run{ID: id, ProjectID: project.ID, BaseCommit: origin, HeadCommit: origin,
			WorkerID: WorkerID(), Directory: filepath.Join(root, id)}
		_, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory)
 VALUES($1,$2,$3,$3,$4,$5)`, id, project.ID, origin, record.WorkerID, record.Directory)
	}
	if err != nil {
		return err
	}
	if record.ProjectID != project.ID || record.WorkerID != WorkerID() || record.Directory != filepath.Join(root, id) || record.BaseCommit != origin {
		return errors.New("重建记录与项目、Worker、磁盘或干净基线不一致")
	}
	if record.ExpiredAt != nil {
		return ErrExpired
	}
	if !record.Ready {
		// 来源已通过 lease/身份/祖先校验；直接克隆其独立 Git 对象，绝不走 P4 或成果导入。
		if err = prepareRun(ctx, conn, record, filepath.Join(source.Directory, "base"), ""); err != nil {
			return err
		}
	}
	base := filepath.Join(record.Directory, "base")
	head, err := git(ctx, base, "rev-parse", "HEAD")
	if err != nil || head != origin {
		return errors.New("重建磁盘不是原始干净基线")
	}
	status, err := git(ctx, base, "status", "--porcelain=v1")
	if err != nil || status != "" {
		return errors.New("重建基线工作区包含未发布变化")
	}
	_, err = git(ctx, base, "cat-file", "-e", record.HeadCommit+"^{commit}")
	return err
}
