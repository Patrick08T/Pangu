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

const versionColumns = `v.id::text,v.object_id::text,o.kind,v.number,coalesce(v.source_revision,0),
 CASE WHEN v.source_kind='manual' THEN v.spec ELSE coalesce((SELECT b.spec FROM pangu.catalog_version_bodies b WHERE b.version_id=v.id),'{}'::jsonb) END,v.created_at,
 CASE WHEN o.merged_into IS NULL THEN o.archived ELSE coalesce((SELECT canonical.archived FROM pangu.catalog_objects canonical WHERE canonical.id=o.merged_into),true) END,
 jsonb_build_object('source_kind',v.source_kind,'project_id',coalesce(v.project_id::text,''),'source_experiment',coalesce(v.source_experiment::text,''),
 'public_baseline',coalesce(v.public_baseline::text,''),'project_previous',coalesce(v.project_previous::text,''),'content_hash',coalesce(v.content_hash,''),
 'state',coalesce((SELECT s.state FROM pangu.catalog_version_state s WHERE s.version_id=v.id),''),
 'tombstoned',v.source_kind='auto' AND NOT EXISTS(SELECT 1 FROM pangu.catalog_version_bodies b WHERE b.version_id=v.id AND b.spec IS NOT NULL))`

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
	result, err := tx.Exec(ctx, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) SELECT $1,$2,coalesce(max(number),0)+1,$3,$4 FROM pangu.catalog_versions WHERE object_id=$2 ON CONFLICT(object_id,source_revision) WHERE source_kind='manual' DO NOTHING`, uuid.NewString(), id, revision, data)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() > 0 {
		if _, err = tx.Exec(ctx, "UPDATE pangu.catalog_objects SET updated_at=now() WHERE id=$1", id); err != nil {
			return nil, err
		}
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
	return scanVersion(reader.QueryRow(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE o.namespace=$1 AND v.id=$2 AND v.source_kind='manual' FOR SHARE OF o", namespace, id))
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
	rows, err := conn.Query(ctx, "SELECT "+versionColumns+" FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE o.namespace=$1 AND o.id=$2 AND v.source_kind='manual' ORDER BY v.number DESC OFFSET $3 LIMIT 100", namespace, id, offset)
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
	var data, metadata []byte
	err := row.Scan(&item.ID, &item.ObjectID, &item.Kind, &item.Number, &item.SourceRevision, &data, &item.CreatedAt, &item.Archived, &metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(metadata, item); err != nil {
		return nil, err
	}
	return item, json.Unmarshal(data, &item.Spec)
}
