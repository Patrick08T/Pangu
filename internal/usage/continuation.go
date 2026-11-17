package usage

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type reader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// 只有平台批准的版本接续共享预算；普通新任务不隐式继承额度。
func budgetRuns(ctx context.Context, db reader, run string) ([]string, error) {
	var ids []string
	err := db.QueryRow(ctx, `WITH RECURSIVE lineage AS (
 SELECT s.id,s.run_id,s.namespace,s.project_id,ARRAY[s.run_id] AS path,0 AS depth FROM pangu.submissions s WHERE s.run_id=$1
 UNION ALL
 SELECT p.id,p.run_id,p.namespace,p.project_id,l.path||p.run_id,l.depth+1
 FROM lineage l JOIN pangu.plan_continuations c ON c.request_id=l.id
 JOIN pangu.approvals a ON a.id=c.approval_id JOIN pangu.submissions p ON p.run_id=a.run_id
 WHERE p.namespace=l.namespace AND p.project_id=l.project_id AND NOT p.run_id=ANY(l.path)
 ) SELECT array_agg(run_id::text ORDER BY depth DESC) FROM lineage`, run).Scan(&ids)
	return ids, err
}
