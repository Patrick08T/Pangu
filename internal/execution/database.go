package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

var ErrCancelled = errors.New("运行已请求取消，禁止启动新的节点尝试")

type Attempt struct {
	RunID             string                     `json:"run_id"`
	NodeRunID         string                     `json:"node_run_id"`
	NodeID            string                     `json:"node_id"`
	Attempt           int                        `json:"attempt"`
	StartedAt         time.Time                  `json:"started_at"`
	FinishedAt        *time.Time                 `json:"finished_at,omitempty"`
	Status            string                     `json:"status"`
	Message           string                     `json:"message"`
	Truncated         bool                       `json:"truncated"`
	Evidence          []workflow.CommandEvidence `json:"evidence"`
	EvidenceTruncated bool                       `json:"evidence_truncated"`
	Category          string                     `json:"category"`
	ProcessScope      string                     `json:"-" db:"-"`
}

func Begin(ctx context.Context, item *Attempt) error {
	if err := Recover(ctx); err != nil {
		return err
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	stamp, err := agent.ProcessIdentity(os.Getpid())
	if err != nil {
		return err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", item.RunID); err != nil {
		return err
	}
	var active int
	if err := tx.QueryRow(ctx, "SELECT coalesce(max(attempt),0)+1,count(*) FILTER(WHERE finished_at IS NULL) FROM pangu.node_attempts WHERE node_run_id=$1", item.NodeRunID).Scan(&item.Attempt, &active); err != nil {
		return err
	}
	if active > 0 {
		return errors.New("上一次尝试仍未确认停止，禁止重叠执行")
	}
	result, err := tx.Exec(ctx, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt,owner_host,owner_pid,owner_stamp,process_scope)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8 WHERE NOT EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1)
		AND NOT EXISTS(SELECT 1 FROM pangu.workspace_runs WHERE run_id=$1 AND expired_at IS NOT NULL)
		AND NOT EXISTS(SELECT 1 FROM pangu.retention_records WHERE run_id=$1 AND status='purged')`,
		item.RunID, item.NodeRunID, item.NodeID, item.Attempt, host, os.Getpid(), stamp, item.ProcessScope)
	if err != nil {
		return errors.New("无法持久记录节点尝试，拒绝启动执行器")
	}
	if result.RowsAffected() == 0 {
		return ErrCancelled
	}
	return tx.Commit(ctx)
}

func Finish(ctx context.Context, item Attempt) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	message, truncated := diagnostic.Text(item.Message, 64*1024)
	if item.Evidence == nil {
		item.Evidence = []workflow.CommandEvidence{}
	}
	evidence, err := json.Marshal(item.Evidence)
	if err != nil {
		return err
	}
	result, err := conn.Exec(ctx, `UPDATE pangu.node_attempts SET finished_at=now(),status=$3,message=$4,truncated=$5,evidence=$6,evidence_truncated=$7,category=$8
		WHERE node_run_id=$1 AND attempt=$2 AND finished_at IS NULL`, item.NodeRunID, item.Attempt, item.Status, message, truncated || item.Truncated, evidence, item.EvidenceTruncated, item.Category)
	if err != nil || result.RowsAffected() != 1 {
		return errors.New("无法持久记录执行器停止证据")
	}
	return nil
}

func Attempts(ctx context.Context, runID string) ([]Attempt, error) {
	page, err := ListAttempts(ctx, runID, 0, 20)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func Active(ctx context.Context, runID string) (int, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())
	var count int
	err = conn.QueryRow(ctx, "SELECT count(*) FROM pangu.node_attempts WHERE run_id=$1 AND finished_at IS NULL", runID).Scan(&count)
	return count, err
}
