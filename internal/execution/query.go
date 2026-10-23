package execution

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type AttemptPage struct {
	Items      []Attempt `json:"items"`
	NextOffset *int      `json:"next_offset,omitempty"`
}

func ParsePage(values url.Values) (int, int, error) {
	offset, limit := 0, 20
	for key, items := range values {
		if len(items) != 1 {
			return 0, 0, errors.New("分页参数不能重复")
		}
		n, err := strconv.Atoi(items[0])
		if err != nil {
			return 0, 0, errors.New("分页参数必须为整数")
		}
		switch key {
		case "offset":
			offset = n
		case "limit":
			limit = n
		default:
			return 0, 0, errors.New("未知分页参数")
		}
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return 0, 0, errors.New("offset 非负，limit 为 1–100")
	}
	return offset, limit, nil
}

func ListAttempts(ctx context.Context, run string, offset, limit int) (*AttemptPage, error) {
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, errors.New("分页无效")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT run_id::text,node_run_id::text,node_id,attempt,started_at,finished_at,status,
		left(message,1024),truncated OR length(message)>1024,'[]'::jsonb,evidence_truncated OR jsonb_array_length(evidence)>0,category
		FROM pangu.node_attempts WHERE run_id=$1 ORDER BY started_at,node_run_id,attempt OFFSET $2 LIMIT $3`, run, offset, limit+1)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Attempt])
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Attempt{}
	}
	out := &AttemptPage{Items: items}
	if len(items) > limit {
		next := offset + limit
		out.NextOffset, out.Items = &next, items[:limit]
	}
	return out, nil
}

func AttemptByID(ctx context.Context, run, node string, attempt int) (*Attempt, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT run_id::text,node_run_id::text,node_id,attempt,started_at,finished_at,status,message,truncated,evidence,evidence_truncated,category
		FROM pangu.node_attempts WHERE run_id=$1 AND node_run_id=$2 AND attempt=$3`, run, node, attempt)
	if err != nil {
		return nil, err
	}
	item, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[Attempt])
	return &item, err
}
