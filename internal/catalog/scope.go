package catalog

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
)

const scopedVersionFilter = `(v.source_kind='manual' OR (v.project_id=$3::uuid
 AND EXISTS(SELECT 1 FROM pangu.catalog_version_bodies b WHERE b.version_id=v.id AND b.spec IS NOT NULL)
 AND EXISTS(SELECT 1 FROM pangu.catalog_version_state s WHERE s.version_id=v.id AND (s.state='stable' OR ($4 AND s.state='observing')))))`

type VersionScope struct {
	ProjectID string
	// AllowObserving 仅供服务端已经授权的实验或观察入口使用，不能来自普通客户端选项。
	AllowObserving bool
}

// ReadVersionScoped 仍需调用边界验证操作者项目权限，scope 不构成授权。
func ReadVersionScoped(ctx context.Context, namespace, id string, scope VersionScope) (*Version, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return readVersionScoped(ctx, conn, namespace, id, scope)
}

func readVersionScoped(ctx context.Context, reader rowReader, namespace, id string, scope VersionScope) (*Version, error) {
	v, err := scanVersion(reader.QueryRow(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE o.namespace=$1 AND v.id=$2 AND "+scopedVersionFilter+" FOR SHARE OF o", namespace, id, nullableID(scope.ProjectID), scope.AllowObserving))
	if err != nil {
		return nil, err
	}
	if err = checkVersionScope(*v, scope); err != nil {
		return nil, err
	}
	return v, nil
}

func VersionsScoped(ctx context.Context, namespace, id string, offset int, scope VersionScope) ([]Version, error) {
	if offset < 0 {
		return nil, invalid("offset 必须为非负整数")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE o.namespace=$1 AND o.id=$2 AND "+scopedVersionFilter+" ORDER BY v.number DESC LIMIT 100 OFFSET $5", namespace, id, nullableID(scope.ProjectID), scope.AllowObserving, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Version{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		if err = checkVersionScope(*v, scope); err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func checkVersionScope(v Version, scope VersionScope) error {
	if v.SourceKind == "" || v.SourceKind == "manual" {
		return nil
	}
	if v.SourceKind != "auto" || scope.ProjectID == "" || scope.ProjectID != v.ProjectID || v.Tombstoned {
		return ErrNotFound
	}
	if v.State == "stable" || (v.State == "observing" && scope.AllowObserving) {
		return nil
	}
	return ErrNotFound
}
