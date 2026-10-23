package execution

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
)

// LatestCategories 从持久尝试读取每个节点的最终分类，不受日志页面大小影响。
func LatestCategories(ctx context.Context, run string) (map[string]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT DISTINCT ON(node_run_id) node_run_id::text,category FROM pangu.node_attempts WHERE run_id=$1 ORDER BY node_run_id,attempt DESC`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var node, category string
		if err := rows.Scan(&node, &category); err != nil {
			return nil, err
		}
		out[node] = category
	}
	return out, rows.Err()
}
