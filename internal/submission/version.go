package submission

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
)

// FindVersion 使用已保存的版本快照核对回执，不因归档而重新编译或接受新输入。
func FindVersion(ctx context.Context, namespace, project, key, version, input string) (*Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	item, _, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND operation='submit' AND key_hash=$3", namespace, project, digest([]byte(key))))
	if err != nil {
		return nil, err
	}
	var hash string
	if err := conn.QueryRow(ctx, "SELECT input_hash FROM pangu.submissions WHERE id=$1", item.ID).Scan(&hash); err != nil {
		return nil, err
	}
	if item.CreatedBy != identity.Current(ctx).ID || item.Snapshot.Workflow.VersionID != version || hash != digest([]byte(input)) {
		return nil, ErrConflict
	}
	return item, nil
}

func FindRerun(ctx context.Context, namespace, project, key, source string) (*Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	item, _, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND operation='rerun' AND key_hash=$3", namespace, project, digest([]byte(key))))
	if err != nil {
		return nil, err
	}
	if item.CreatedBy != identity.Current(ctx).ID || item.Snapshot.SourceRunID != source {
		return nil, ErrConflict
	}
	return item, nil
}

func FindFork(ctx context.Context, namespace, project, key, source, requestHash string) (*Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	item, _, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND operation='fork' AND key_hash=$3", namespace, project, digest([]byte(key))))
	if err != nil {
		return nil, err
	}
	if item.CreatedBy != identity.Current(ctx).ID || item.Snapshot.SourceRunID != source || item.Snapshot.Fork == nil || item.Snapshot.Fork.RequestHash != requestHash {
		return nil, ErrConflict
	}
	return item, nil
}
