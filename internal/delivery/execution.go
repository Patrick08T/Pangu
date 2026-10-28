package delivery

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
)

func Verified(ctx context.Context, namespace, id string) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	var count int
	err = conn.QueryRow(ctx, `SELECT count(DISTINCT a.node_id) FROM pangu.deliveries d JOIN pangu.submissions s ON s.id=d.request_id
 JOIN pangu.node_attempts a ON a.run_id=s.run_id JOIN pangu.workspace_checkpoints c ON c.node_run_id=a.node_run_id
 WHERE d.namespace=$1 AND d.id=$2 AND a.node_id IN ('implementation','review') AND a.status='succeeded'
 AND c.evidence @> '[{"id":"project-verification","exit_code":0}]'::jsonb`, namespace, id).Scan(&count)
	return count == 2, err
}
