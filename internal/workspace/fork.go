package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func EnsureFork(ctx context.Context, id, project, namespace, source, commit string) error {
	if id == source || source == "" {
		return errors.New("返工必须创建独立的新运行")
	}
	return ensure(ctx, id, project, namespace, source, commit)
}

func forkCommit(ctx context.Context, source, commit string) (string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	var exists bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.workspace_runs r LEFT JOIN pangu.submissions s ON s.run_id=r.run_id WHERE r.run_id=$1 AND r.expired_at IS NULL AND (r.base_commit=$2 OR s.snapshot#>>'{fork,origin_commit}'=$2 OR EXISTS(SELECT 1 FROM pangu.workspace_checkpoints c WHERE c.run_id=r.run_id AND c.commit_id=$2)))`, source, commit).Scan(&exists)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", errors.New("来源成功检查点不存在或已过期")
	}
	return commit, nil
}

func ImportCheckpoints(ctx context.Context, run string, fork *workflow.Fork) (map[string]workflow.NodeOutput, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	out := map[string]workflow.NodeOutput{}
	for node, value := range fork.Reused {
		commit := fork.Checkpoints[node]
		if commit == "" {
			return nil, errors.New("复用节点缺少文件检查点")
		}
		value.RunID = uuid.NewSHA1(uuid.MustParse(run), []byte(node)).String()
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		_, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_checkpoints(node_run_id,run_id,commit_id,output) VALUES($1,$2,$3,$4) ON CONFLICT(node_run_id) DO NOTHING`, value.RunID, run, commit, raw)
		if err != nil {
			return nil, err
		}
		out[node] = value
	}
	return out, nil
}

func Checkpoint(ctx context.Context, source, nodeRun string) (string, workflow.NodeOutput, error) {
	var out workflow.NodeOutput
	conn, err := database.Open(ctx)
	if err != nil {
		return "", out, err
	}
	defer conn.Close(context.Background())
	var raw []byte
	var commit string
	err = conn.QueryRow(ctx, `SELECT c.commit_id,c.output FROM pangu.workspace_checkpoints c JOIN pangu.workspace_runs r ON r.run_id=c.run_id WHERE c.run_id=$1 AND c.node_run_id=$2 AND r.expired_at IS NULL`, source, nodeRun).Scan(&commit, &raw)
	if err != nil {
		return "", out, err
	}
	if diagnostic.Sensitive(raw) {
		return "", out, errors.New("检查点输出疑似包含敏感数据")
	}
	err = json.Unmarshal(raw, &out)
	return commit, out, err
}
