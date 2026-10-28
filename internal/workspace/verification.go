package workspace

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
)

func Verification(ctx context.Context, run string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	var command []string
	err = conn.QueryRow(ctx, `SELECT p.verification FROM pangu.projects p JOIN pangu.workspace_runs r ON r.project_id=p.id WHERE r.run_id=$1 AND r.worker_id=$2 AND r.expired_at IS NULL`, run, WorkerID()).Scan(&command)
	return command, err
}
