package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SaveReport 随调用方事务保存正文与引用，避免检查点成功但报告丢失。
func SaveReport(ctx context.Context, tx pgx.Tx, run, node string, out workflow.NodeOutput) error {
	if len(out.Reports) == 0 && len(out.ReportIDs) == 0 {
		return nil
	}
	if len(out.Reports) != 1 || len(out.ReportIDs) != 1 || out.ReportIDs[0] != out.Reports[0].ID || out.RunID != node {
		return errors.New("报告与节点结果不一致")
	}
	report := out.Reports[0]
	if _, err := uuid.Parse(report.ID); err != nil || len(report.Markdown) == 0 || len(report.Markdown) > 1024*1024 || !utf8.ValidString(report.Markdown) {
		return errors.New("报告 ID、编码或大小无效")
	}
	if diagnostic.Sensitive([]byte(report.Markdown)) || diagnostic.Sensitive([]byte(report.Title)) || diagnostic.Sensitive(report.Metadata) {
		return errors.New("报告包含敏感信息")
	}
	if out.ReportSHA256 != "" && out.ReportSHA256 != workflow.ReportHash(report.Markdown) {
		return errors.New("报告内容与摘要不一致")
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	evidence, err := json.Marshal(out.Evidence)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence,metadata) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		report.ID, run, node, report.Title, []byte(report.Markdown), data, evidence, report.Metadata)
	return err
}

func copyReport(ctx context.Context, tx pgx.Tx, run, node, commit, reportHash string, out workflow.NodeOutput) (workflow.NodeOutput, error) {
	originalNode := out.RunID
	out.RunID = node
	if len(out.ReportIDs) == 0 {
		return out, nil
	}
	if commit != "" {
		reportHash = out.ReportSHA256
	}
	if len(out.ReportIDs) != 1 || reportHash == "" {
		return out, errors.New("返工报告缺少唯一引用或内容摘要")
	}
	report := workflow.Report{ID: uuid.NewSHA1(uuid.MustParse(node), []byte("report")).String()}
	var body []byte
	err := tx.QueryRow(ctx, `SELECT r.title,r.body,r.metadata FROM pangu.research_reports r
 JOIN pangu.workspace_runs source ON source.run_id=r.run_id
 JOIN pangu.workspace_runs target ON target.run_id=$1 AND target.project_id=source.project_id
 WHERE r.id=$2 AND r.node_run_id=$3 AND source.expired_at IS NULL AND target.expired_at IS NULL
 AND ($4='' OR EXISTS(SELECT 1 FROM pangu.workspace_checkpoints c WHERE c.run_id=r.run_id AND c.node_run_id=r.node_run_id AND c.commit_id=$4))`,
		run, out.ReportIDs[0], originalNode, commit).Scan(&report.Title, &body, &report.Metadata)
	if err != nil {
		return out, errors.New("来源报告与项目或成功检查点不匹配")
	}
	report.Markdown = string(body)
	if workflow.ReportHash(report.Markdown) != reportHash {
		return out, errors.New("来源报告内容已变化")
	}
	out.ReportIDs, out.Reports = []string{report.ID}, []workflow.Report{report}
	return out, nil
}
