package catalog

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/Patrick08T/Pangu/internal/database"
)

func ResolveConnection(ctx context.Context, project, id string) (*Connection, string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, "", err
	}
	defer conn.Close(context.Background())
	item, err := scanVersion(conn.QueryRow(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE v.id=$1 AND o.project_id=$2 AND o.kind='connection'", id, project))
	if err != nil {
		return nil, "", err
	}
	if item.Archived {
		return nil, "", ErrArchived
	}
	c := item.Spec.Connection
	if c == nil {
		return nil, "", errors.New("连接配置缺失")
	}
	if err := c.Validate(); err != nil {
		return nil, "", err
	}
	if err := c.Granted(project); err != nil {
		return nil, "", err
	}
	key := os.Getenv(c.SecretRef)
	if strings.TrimSpace(key) == "" {
		return nil, "", errors.New("服务端凭据引用未配置")
	}
	return c, key, nil
}
