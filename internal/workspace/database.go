package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Run struct {
	ID         string     `json:"run_id"`
	ProjectID  string     `json:"project_id"`
	BaseCommit string     `json:"base_commit"`
	HeadCommit string     `json:"head_commit"`
	WorkerID   string     `json:"worker_id"`
	Directory  string     `json:"-"`
	Ready      bool       `json:"ready"`
	ExpiredAt  *time.Time `json:"expired_at,omitempty"`
}

func lockRun(ctx context.Context, runID string) (*pgx.Conn, error) {
	if _, err := uuid.Parse(runID); err != nil {
		return nil, errors.New("运行 ID 无效")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", "workspace:"+runID); err != nil {
		_ = conn.Close(context.Background())
		return nil, err
	}
	return conn, nil
}

func readRun(ctx context.Context, conn *pgx.Conn, id string) (*Run, error) {
	out := &Run{ID: id}
	err := conn.QueryRow(ctx, `SELECT project_id::text,base_commit,head_commit,worker_id,directory,ready,expired_at
		FROM pangu.workspace_runs WHERE run_id=$1`, id).
		Scan(&out.ProjectID, &out.BaseCommit, &out.HeadCommit, &out.WorkerID, &out.Directory, &out.Ready, &out.ExpiredAt)
	return out, err
}

func Get(ctx context.Context, id string) (*Run, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return readRun(ctx, conn, id)
}

func Ensure(ctx context.Context, runID, projectID, namespace string) error {
	return ensure(ctx, runID, projectID, namespace, "", "")
}

func EnsureRerun(ctx context.Context, runID, projectID, namespace, sourceRunID string) error {
	if runID == sourceRunID || sourceRunID == "" {
		return errors.New("重跑必须使用新的运行 ID")
	}
	return ensure(ctx, runID, projectID, namespace, sourceRunID, "")
}

func ensure(ctx context.Context, runID, projectID, namespace, sourceRunID, sourceCommit string) error {
	project, err := ProjectByID(ctx, namespace, projectID)
	if err != nil {
		return err
	}
	if project.WorkerID != WorkerID() {
		return errors.New("项目绑定其他 Worker，禁止自动接管本地磁盘")
	}
	root, err := workspaceRoot()
	if err != nil {
		return err
	}
	conn, err := lockRun(ctx, runID)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if expired, err := Expired(ctx, runID); err != nil || expired {
		if err != nil {
			return err
		}
		return ErrExpired
	}
	record, err := readRun(ctx, conn, runID)
	if errors.Is(err, pgx.ErrNoRows) {
		if sourceRunID != "" {
			release, err := Lease(ctx, sourceRunID)
			if err != nil {
				return err
			}
			defer release()
		}
		record, err = createRun(ctx, conn, runID, root, project, sourceRunID, sourceCommit)
	}
	if err != nil {
		return err
	}
	if record.ProjectID != project.ID || record.WorkerID != WorkerID() || record.Directory != filepath.Join(root, runID) {
		return errors.New("运行与项目、Worker 或磁盘记录不一致")
	}
	if record.ExpiredAt != nil {
		return ErrExpired
	}
	if !record.Ready {
		return prepareRun(ctx, conn, record, project.Source, sourceRunID)
	}
	_, err = git(ctx, filepath.Join(record.Directory, "base"), "cat-file", "-e", record.HeadCommit+"^{commit}")
	return err
}

func createRun(ctx context.Context, conn *pgx.Conn, id, root string, project *Project, sourceRunID, sourceCommit string) (*Run, error) {
	commit, err := initialCommit(ctx, project, sourceRunID)
	if err == nil && sourceCommit != "" {
		commit, err = forkCommit(ctx, sourceRunID, sourceCommit)
	}
	if err != nil {
		return nil, err
	}
	record := &Run{ID: id, ProjectID: project.ID, BaseCommit: commit, HeadCommit: commit,
		WorkerID: WorkerID(), Directory: filepath.Join(root, id)}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory)
		VALUES($1,$2,$3,$3,$4,$5)`, id, project.ID, commit, record.WorkerID, record.Directory)
	return record, err
}

func prepareRun(ctx context.Context, conn *pgx.Conn, record *Run, source, sourceRunID string) error {
	base := filepath.Join(record.Directory, "base")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		if sourceRunID != "" {
			release, err := Lease(ctx, sourceRunID)
			if err != nil {
				return err
			}
			defer release()
			previous, err := RerunSource(ctx, sourceRunID, record.ProjectID)
			if err != nil {
				return err
			}
			if previous.BaseCommit != record.BaseCommit {
				if _, err := forkCommit(ctx, sourceRunID, record.BaseCommit); err != nil {
					return err
				}
			}
			source = filepath.Join(previous.Directory, "base")
		}
		staging := filepath.Join(record.Directory, "preparing-"+uuid.NewString())
		if err := Clone(ctx, source, record.BaseCommit, staging); err != nil {
			return err
		}
		if err := os.Rename(staging, base); err != nil {
			return err
		}
	}
	commit, err := git(ctx, base, "rev-parse", "HEAD")
	if err != nil || commit != record.BaseCommit {
		return errors.New("运行基准缺失或准备状态不一致")
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.workspace_runs SET ready=true WHERE run_id=$1", record.ID)
	return err
}

func Execute(ctx context.Context, runID, nodeRunID string, attempt int, readOnly bool, execute func(string) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	conn, err := lockRun(ctx, runID)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	defer conn.Close(context.Background())
	record, err := readRun(ctx, conn, runID)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	if record.ExpiredAt != nil {
		return workflow.NodeOutput{}, ErrExpired
	}
	root, err := workspaceRoot()
	if err != nil || record.WorkerID != WorkerID() || record.Directory != filepath.Join(root, runID) || !record.Ready {
		return workflow.NodeOutput{}, errors.New("运行工作区不可用或属于其他 Worker/磁盘")
	}
	var output []byte
	var evidence []byte
	var truncated bool
	var checkpoint string
	err = conn.QueryRow(ctx, "SELECT output,commit_id,evidence,evidence_truncated FROM pangu.workspace_checkpoints WHERE node_run_id=$1 AND run_id=$2", nodeRunID, runID).Scan(&output, &checkpoint, &evidence, &truncated)
	if err == nil {
		if _, err := git(ctx, filepath.Join(record.Directory, "base"), "cat-file", "-e", checkpoint+"^{commit}"); err != nil {
			return workflow.NodeOutput{}, errors.New("已完成节点的文件检查点丢失")
		}
		var result workflow.NodeOutput
		err = json.Unmarshal(output, &result)
		if err == nil {
			err = json.Unmarshal(evidence, &result.Evidence)
		}
		result.EvidenceTruncated = truncated
		return result, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return workflow.NodeOutput{}, err
	}
	return executeAttempt(ctx, conn, record, nodeRunID, attempt, readOnly, execute)
}

func executeAttempt(ctx context.Context, conn *pgx.Conn, record *Run, nodeRunID string, attempt int, readOnly bool, execute func(string) (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	if _, err := uuid.Parse(nodeRunID); err != nil || attempt < 1 {
		return workflow.NodeOutput{}, errors.New("节点运行或尝试编号无效")
	}
	base := filepath.Join(record.Directory, "base")
	path := filepath.Join(record.Directory, "attempt-"+uuid.NewString())
	_, err := conn.Exec(ctx, `INSERT INTO pangu.workspace_attempts(run_id,node_run_id,attempt,directory,base_commit,status)
		VALUES($1,$2,$3,$4,$5,'preparing')`, record.ID, nodeRunID, attempt, path, record.HeadCommit)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	defer failAttempt(nodeRunID, attempt)
	if err := Clone(ctx, base, record.HeadCommit, path); err != nil {
		return workflow.NodeOutput{}, err
	}
	if _, err := conn.Exec(ctx, "UPDATE pangu.workspace_attempts SET status='running' WHERE node_run_id=$1 AND attempt=$2", nodeRunID, attempt); err != nil {
		return workflow.NodeOutput{}, err
	}
	out, err := execute(path)
	if err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	commit, err := Snapshot(ctx, base, path, record.HeadCommit)
	if err != nil {
		return out, err
	}
	if readOnly {
		changed, err := git(ctx, base, "diff", "--name-only", "--no-ext-diff", record.HeadCommit, commit, "--")
		if err != nil {
			return out, err
		}
		if changed != "" {
			return out, &workflow.Failure{Kind: "permission", Cause: errors.New("只读节点产生文件修改，拒绝发布检查点")}
		}
	}
	return out, publish(ctx, conn, record, nodeRunID, commit, attempt, out)
}

func publish(ctx context.Context, conn *pgx.Conn, record *Run, nodeRunID, commit string, attempt int, out workflow.NodeOutput) error {
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if out.Evidence == nil {
		out.Evidence = []workflow.CommandEvidence{}
	}
	evidence, err := json.Marshal(out.Evidence)
	if err != nil {
		return err
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", record.ID); err != nil {
		return err
	}
	var cancelled bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1)", record.ID).Scan(&cancelled); err != nil {
		return err
	}
	if cancelled {
		return errors.New("运行已请求取消，拒绝发布文件检查点")
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.workspace_checkpoints(node_run_id,run_id,commit_id,output,evidence,evidence_truncated) VALUES($1,$2,$3,$4,$5,$6)`, nodeRunID, record.ID, commit, data, evidence, out.EvidenceTruncated)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, "UPDATE pangu.workspace_runs SET head_commit=$2 WHERE run_id=$1 AND head_commit=$3", record.ID, commit, record.HeadCommit)
	if err != nil || result.RowsAffected() != 1 {
		return errors.New("检查点发布冲突，未改变运行基准")
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.workspace_attempts SET status='published' WHERE node_run_id=$1 AND attempt=$2", nodeRunID, attempt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
