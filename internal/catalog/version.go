package catalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type rowReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const versionColumns = `v.id::text,v.object_id::text,o.kind,v.number,v.source_revision,v.spec,v.created_at,
 CASE WHEN o.merged_into IS NULL THEN o.archived ELSE coalesce((SELECT canonical.archived FROM pangu.catalog_objects canonical WHERE canonical.id=o.merged_into),true) END`

func Publish(ctx context.Context, namespace, id string, revision int) (*Version, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	version, err := publishObject(ctx, tx, namespace, id, revision)
	if err != nil {
		return nil, err
	}
	return version, tx.Commit(ctx)
}

func publishObject(ctx context.Context, tx pgx.Tx, namespace, id string, revision int) (*Version, error) {
	item, err := lockedObject(ctx, tx, namespace, id, revision)
	if err != nil {
		return nil, err
	}
	if err := item.NewObject.Validate(); err != nil {
		return nil, invalid(err.Error())
	}
	if err := validateReferences(ctx, tx, namespace, item.NewObject, true); err != nil {
		return nil, err
	}
	data, err := json.Marshal(item.Spec)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) SELECT $1,$2,coalesce(max(number),0)+1,$3,$4 FROM pangu.catalog_versions WHERE object_id=$2 ON CONFLICT(object_id,source_revision) DO NOTHING`, uuid.NewString(), id, revision, data)
	if err != nil {
		return nil, err
	}
	version, err := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE o.id=$1 AND v.source_revision=$2", id, revision))
	if err != nil {
		return nil, err
	}
	return version, nil
}

func ReadVersion(ctx context.Context, namespace, id string) (*Version, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return readVersion(ctx, conn, namespace, id)
}

func readVersion(ctx context.Context, reader rowReader, namespace, id string) (*Version, error) {
	return scanVersion(reader.QueryRow(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE o.namespace=$1 AND v.id=$2 FOR SHARE OF o", namespace, id))
}

func Versions(ctx context.Context, namespace, id string, offset int) ([]Version, error) {
	if offset < 0 {
		return nil, invalid("offset 必须为非负整数")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE o.namespace=$1 AND o.id=$2 ORDER BY v.number DESC OFFSET $3 LIMIT 100", namespace, id, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Version{}
	for rows.Next() {
		item, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanVersion(row pgx.Row) (*Version, error) {
	item := &Version{}
	var data []byte
	err := row.Scan(&item.ID, &item.ObjectID, &item.Kind, &item.Number, &item.SourceRevision, &data, &item.CreatedAt, &item.Archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, json.Unmarshal(data, &item.Spec)
}
