package research

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/jackc/pgx/v5"
)

func Persist(ctx context.Context, run, node, project string, execute func() (workflow.NodeOutput, error)) (workflow.NodeOutput, error) {
	record, err := workspace.Get(ctx, run)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	if record.ProjectID != project || record.WorkerID != workspace.WorkerID() || !record.Ready {
		return workflow.NodeOutput{}, errors.New("报告运行与项目或 Worker 不匹配")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", "research:"+node); err != nil {
		return workflow.NodeOutput{}, err
	}
	var raw, evidence []byte
	err = conn.QueryRow(ctx, "SELECT output,evidence FROM pangu.research_reports WHERE run_id=$1 AND node_run_id=$2", run, node).Scan(&raw, &evidence)
	if err == nil {
		var out workflow.NodeOutput
		if err = json.Unmarshal(raw, &out); err == nil {
			err = json.Unmarshal(evidence, &out.Evidence)
		}
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return workflow.NodeOutput{}, err
	}
	out, err := execute()
	if err != nil {
		return out, err
	}
	return out, save(ctx, conn, run, node, out)
}

func save(ctx context.Context, conn *pgx.Conn, run, node string, out workflow.NodeOutput) error {
	if len(out.Reports) != 1 {
		return errors.New("研究节点必须生成完整报告")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", run); err != nil {
		return err
	}
	var cancelled bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1)", run).Scan(&cancelled); err != nil {
		return err
	}
	if cancelled {
		return errors.New("运行已取消，拒绝发布报告")
	}
	if err = workspace.SaveReport(ctx, tx, run, node, out); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Record struct {
	ID, Title string
	Body      []byte
}

// Checkpoint 读取已原子保存的研究结果；研究不产生文件提交。
func Checkpoint(ctx context.Context, run, node string) (workflow.NodeOutput, error) {
	var out workflow.NodeOutput
	conn, err := database.Open(ctx)
	if err != nil {
		return out, err
	}
	defer conn.Close(context.Background())
	var raw, body []byte
	var report workflow.Report
	err = conn.QueryRow(ctx, `SELECT r.output,r.id::text,r.title,r.body FROM pangu.research_reports r JOIN pangu.workspace_runs w ON w.run_id=r.run_id
 WHERE r.run_id=$1 AND r.node_run_id=$2 AND w.expired_at IS NULL`, run, node).Scan(&raw, &report.ID, &report.Title, &body)
	if err == nil {
		err = json.Unmarshal(raw, &out)
		report.Markdown = string(body)
		out.Reports = []workflow.Report{report}
	}
	return out, err
}

func Reports(ctx context.Context, run string) ([]Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT id::text,title,body FROM pangu.research_reports WHERE run_id=$1 ORDER BY created_at,id LIMIT 33`, run)
	if err != nil {
		return nil, err
	}
	records, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Record])
	if len(records) > 32 {
		return nil, errors.New("报告超过节点数上限")
	}
	return records, err
}
