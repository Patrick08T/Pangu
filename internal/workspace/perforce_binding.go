package workspace

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Patrick08T/Pangu/internal/database"
)

func BindPerforceDirectory(ctx context.Context, namespace, id, previous, source string) (*Project, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `UPDATE pangu.projects SET source=$4 WHERE namespace=$1 AND id=$2 AND vcs='perforce' AND source=$3 AND NOT EXISTS(SELECT 1 FROM pangu.submissions s WHERE s.project_id=$2 AND s.finished_at IS NULL)`, namespace, id, previous, filepath.Clean(source))
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() != 1 {
		return nil, errors.New("项目存在未结束运行、目录已被修改或并非 Perforce 项目，请刷新后再试")
	}
	return ProjectByID(ctx, namespace, id)
}
