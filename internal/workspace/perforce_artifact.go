package workspace

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func localArtifactHead(ctx context.Context, id string) (string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	var head string
	err = conn.QueryRow(ctx, "SELECT local_commit FROM pangu.perforce_sources WHERE run_id=$1 AND local_path<>''", id).Scan(&head)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return head, err
}
