package research

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
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
	if len(out.Reports) != 1 || len(out.ReportIDs) != 1 || out.ReportIDs[0] != out.Reports[0].ID || out.RunID != node {
		return errors.New("报告与节点结果不一致")
	}
	report := out.Reports[0]
	if _, err := uuid.Parse(report.ID); err != nil || len(report.Markdown) > 1024*1024 {
		return errors.New("报告 ID 或大小无效")
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
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
	_, err = tx.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence,metadata) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, report.ID, run, node, report.Title, []byte(report.Markdown), data, evidence, report.Metadata)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Record struct {
	ID, Title string
	Body      []byte
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
