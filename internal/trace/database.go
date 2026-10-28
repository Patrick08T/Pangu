package trace

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func Append(ctx context.Context, event Event) error {
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
	// 同一运行串行提交序号，防止增量读取越过尚未提交的较小序号。
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "trace:"+event.RunID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `INSERT INTO pangu.trace_events(run_id,node_id,node_run_id,attempt,span_id,parent_span_id,kind,phase,name,occurred_at,duration_ms,status,input,output,error,truncated)
	 SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16
	 WHERE NOT EXISTS(SELECT 1 FROM pangu.retention_records WHERE run_id=$1 AND status='purged')
	 AND NOT EXISTS(SELECT 1 FROM pangu.run_debug WHERE run_id=$1 AND expired)
	 AND NOT EXISTS(SELECT 1 FROM pangu.workspace_runs WHERE run_id=$1 AND expired_at IS NOT NULL)`,
		event.RunID, event.NodeID, event.NodeRunID, event.Attempt, event.SpanID, event.ParentID, event.Kind, event.Phase,
		event.Name, event.Time, event.Duration, event.Status, event.Input, event.Output, event.Error, event.Truncated)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("运行正文已清理，不能恢复追踪内容")
	}
	return tx.Commit(ctx)
}

type Page struct {
	Items      []Event `json:"items"`
	NextBefore *int64  `json:"next_before,omitempty"`
}

func List(ctx context.Context, run string, query Query) (*Page, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT id,run_id::text,node_id,node_run_id::text,attempt,span_id,parent_span_id,kind,phase,name,occurred_at,duration_ms,status,input,output,error,truncated
	 FROM pangu.trace_events WHERE run_id=$1 AND id>$2 AND ($3::bigint=0 OR id<$3)
	 AND ($4='' OR node_id=$4) AND ($5='' OR kind=$5) AND ($6='' OR status=$6) ORDER BY id DESC LIMIT $7`,
		run, query.After, query.Before, query.NodeID, query.Kind, query.Status, query.Limit+1)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Event])
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Event{}
	}
	out := &Page{Items: items}
	if len(items) > query.Limit {
		cursor := items[query.Limit-1].ID
		out.NextBefore, out.Items = &cursor, items[:query.Limit]
	}
	return out, nil
}

// Activity 返回每个活动节点最近开始且尚未结束的操作，不受历史分页影响。
func Activity(ctx context.Context, run string) ([]Event, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `WITH latest AS (
	 SELECT DISTINCT ON (e.node_run_id,e.attempt,e.span_id) e.id,e.node_id,e.phase
	 FROM pangu.trace_events e JOIN pangu.node_attempts a
	 ON a.run_id=e.run_id AND a.node_run_id=e.node_run_id AND a.attempt=e.attempt
	 WHERE a.run_id=$1 AND a.finished_at IS NULL
	 ORDER BY e.node_run_id,e.attempt,e.span_id,e.id DESC
	), current AS (
	 SELECT DISTINCT ON (node_id) id FROM latest WHERE phase='start' ORDER BY node_id,id DESC
	)
	 SELECT e.id,run_id::text,node_id,node_run_id::text,attempt,span_id,parent_span_id,
	 kind,phase,name,occurred_at,duration_ms,status,input,output,error,truncated
	 FROM current c JOIN pangu.trace_events e ON e.id=c.id ORDER BY node_id`, run)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Event])
	if items == nil {
		items = []Event{}
	}
	return items, err
}
