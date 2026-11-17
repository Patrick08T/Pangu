package approval

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"time"
)

type Summary struct {
	ApprovalGeneration int        `json:"approval_generation"`
	InvalidationReason string     `json:"invalidation_reason,omitempty"`
	InvalidatedAt      *time.Time `json:"invalidated_at,omitempty"`
	ID                 string     `json:"id"`
	RunID              string     `json:"run_id"`
	NodeID             string     `json:"node_id"`
	Title              string     `json:"title"`
	Status             string     `json:"status"`
	Round              int        `json:"round"`
	CreatedAt          time.Time  `json:"created_at"`
}
type Page struct {
	Items      []Summary `json:"items"`
	NextOffset *int      `json:"next_offset,omitempty"`
}
type Query struct {
	ProjectID, RunID, NodeID, Status string
	Offset, Limit                    int
}

func Browse(ctx context.Context, namespace string, q Query) (*Page, error) {
	if q.ProjectID == "" || q.Offset < 0 || q.Limit < 1 || q.Limit > 100 {
		return nil, ErrInvalid
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT a.id::text,a.run_id::text,a.node_id,coalesce(a.subject#>>'{review,title}','历史整体审核'),a.status,a.round,a.created_at,a.approval_generation,a.invalidation_reason,a.invalidated_at
 FROM pangu.approvals a WHERE a.namespace=$1 AND a.project_id=$2 AND ($3='' OR a.run_id::text=$3) AND ($4='' OR a.node_id=$4)
 AND ($5='' OR a.status=$5) AND ($3<>'' OR NOT EXISTS(SELECT 1 FROM pangu.approvals n WHERE n.run_id=a.run_id AND n.node_id=a.node_id AND (n.approval_generation,n.round)>(a.approval_generation,a.round)))
 ORDER BY a.created_at DESC,a.id DESC OFFSET $6 LIMIT $7`, namespace, q.ProjectID, q.RunID, q.NodeID, q.Status, q.Offset, q.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &Page{Items: []Summary{}}
	for rows.Next() {
		var item Summary
		if err = rows.Scan(&item.ID, &item.RunID, &item.NodeID, &item.Title, &item.Status, &item.Round, &item.CreatedAt, &item.ApprovalGeneration, &item.InvalidationReason, &item.InvalidatedAt); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, item)
	}
	if len(page.Items) > q.Limit {
		next := q.Offset + q.Limit
		page.NextOffset = &next
		page.Items = page.Items[:q.Limit]
	}
	return page, rows.Err()
}
