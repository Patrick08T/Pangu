package evaluation

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
	"time"
)

type CaseRun struct {
	CaseID    string    `json:"case_id"`
	VersionID string    `json:"version_id"`
	RequestID string    `json:"request_id"`
	RunID     string    `json:"run_id"`
	Decision  string    `json:"decision"`
	CreatedAt time.Time `json:"created_at"`
}

func LinkRun(ctx context.Context, namespace, id, version, request string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `INSERT INTO pangu.evaluation_runs(case_id,version_id,request_id)
	 SELECT c.id,$3::uuid,s.id FROM pangu.evaluation_cases c JOIN pangu.submissions s ON s.namespace=c.namespace AND s.project_id=c.project_id AND s.input_hash=c.input_hash
	 WHERE c.namespace=$1 AND c.id=$2 AND s.id=$4 AND s.snapshot#>>'{workflow,version_id}'=$3::text
 ON CONFLICT(case_id,request_id) DO UPDATE SET request_id=excluded.request_id WHERE pangu.evaluation_runs.version_id=excluded.version_id`, namespace, id, version, request)
	if err == nil && result.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}

func CaseRuns(ctx context.Context, namespace, id string, offset int) ([]CaseRun, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT e.case_id::text,e.version_id::text,e.request_id::text,coalesce(s.run_id::text,''),coalesce(f.decision,''),e.created_at
 FROM pangu.evaluation_runs e JOIN pangu.evaluation_cases c ON c.id=e.case_id JOIN pangu.submissions s ON s.id=e.request_id LEFT JOIN pangu.feedback f ON f.run_id=s.run_id AND f.source='feishu_session'
 WHERE c.namespace=$1 AND c.id=$2 ORDER BY e.created_at DESC,e.request_id DESC LIMIT 100 OFFSET $3`, namespace, id, offset)
	if err != nil {
		return nil, err
	}
	result, err := pgx.CollectRows(rows, pgx.RowToStructByPos[CaseRun])
	if result == nil && err == nil {
		result = []CaseRun{}
	}
	return result, err
}
