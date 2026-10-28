package usage

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"time"
)

type Call struct {
	ID             string          `json:"id"`
	NodeID         string          `json:"node_id"`
	NodeRunID      string          `json:"node_run_id"`
	Attempt        int             `json:"attempt"`
	Kind           string          `json:"kind"`
	Model          string          `json:"model"`
	Price          *workflow.Price `json:"price"`
	Status         string          `json:"status"`
	InputTokens    *int64          `json:"input_tokens"`
	OutputTokens   *int64          `json:"output_tokens"`
	CachedTokens   *int64          `json:"cached_tokens"`
	EstimateMicros *int64          `json:"estimate_micros"`
	CreatedAt      time.Time       `json:"created_at"`
	FinishedAt     *time.Time      `json:"finished_at"`
}

type Report struct {
	Budget     *workflow.Budget `json:"budget"`
	Used       workflow.Budget  `json:"used"`
	Calls      []Call           `json:"calls"`
	NextOffset *int             `json:"next_offset,omitempty"`
}

func ForRun(ctx context.Context, run string, offset ...int) (*Report, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	out := &Report{Calls: []Call{}}
	var raw []byte
	if err = conn.QueryRow(ctx, "SELECT snapshot->'workflow' FROM pangu.submissions WHERE run_id=$1", run).Scan(&raw); err != nil {
		return nil, err
	}
	var d workflow.Definition
	if err = json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	out.Budget = d.Budget
	err = conn.QueryRow(ctx, `SELECT count(*) FILTER(WHERE kind='model'),count(*) FILTER(WHERE kind='search'),count(*) FILTER(WHERE kind='codex') FROM pangu.usage_calls WHERE run_id=$1`, run).Scan(&out.Used.ModelCalls, &out.Used.SearchCalls, &out.Used.CodexSessions)
	if err != nil {
		return nil, err
	}
	page := 0
	if len(offset) > 0 {
		page = offset[0]
	}
	rows, err := conn.Query(ctx, `SELECT id::text,node_id,node_run_id::text,attempt,kind,model,price,status,input_tokens,output_tokens,cached_tokens,estimate_micros,created_at,finished_at FROM pangu.usage_calls WHERE run_id=$1 ORDER BY created_at,id LIMIT 100 OFFSET $2`, run, page)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Call
		var rates []byte
		err = rows.Scan(&item.ID, &item.NodeID, &item.NodeRunID, &item.Attempt, &item.Kind, &item.Model, &rates, &item.Status, &item.InputTokens, &item.OutputTokens, &item.CachedTokens, &item.EstimateMicros, &item.CreatedAt, &item.FinishedAt)
		if err != nil {
			return nil, err
		}
		if len(rates) > 0 {
			if err = json.Unmarshal(rates, &item.Price); err != nil {
				return nil, err
			}
		}
		out.Calls = append(out.Calls, item)
	}
	if len(out.Calls) == 100 {
		next := page + 100
		out.NextOffset = &next
	}
	return out, rows.Err()
}

func Maintain(ctx context.Context) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `DELETE FROM pangu.usage_calls u WHERE u.created_at<now()-interval '90 days' AND NOT EXISTS(SELECT 1 FROM pangu.node_attempts a WHERE a.run_id=u.run_id AND a.status='running')`)
	return err
}
