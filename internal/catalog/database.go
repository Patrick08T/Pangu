package catalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const objectColumns = `id::text,kind,coalesce(project_id::text,''),name,revision,archived,spec,created_at`

func Create(ctx context.Context, namespace string, input NewObject) (*Object, error) {
	if err := input.Validate(); err != nil {
		return nil, invalid(err.Error())
	}
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
	if err := validateReferences(ctx, tx, namespace, input, false, ""); err != nil {
		return nil, err
	}
	data, err := json.Marshal(input.Spec)
	if err != nil {
		return nil, err
	}
	item, err := scanObject(tx.QueryRow(ctx, `INSERT INTO pangu.catalog_objects(id,namespace,kind,project_id,name,spec) VALUES($1,$2,$3,nullif($4,'')::uuid,$5,$6) RETURNING `+objectColumns, uuid.NewString(), namespace, input.Kind, input.ProjectID, input.Name, data))
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

func Get(ctx context.Context, namespace, id string) (*Object, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scanObject(conn.QueryRow(ctx, "SELECT "+objectColumns+" FROM pangu.catalog_objects WHERE namespace=$1 AND id=$2", namespace, id))
}

func List(ctx context.Context, namespace, kind, project string, offset int) ([]Object, error) {
	if offset < 0 {
		return nil, errors.New("offset 必须为非负整数")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+objectColumns+" FROM pangu.catalog_objects WHERE namespace=$1 AND ($2='' OR kind=$2) AND ($3='' OR project_id=nullif($3,'')::uuid) ORDER BY created_at,id OFFSET $4 LIMIT 100", namespace, kind, project, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Object{}
	for rows.Next() {
		item, err := scanObject(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func Save(ctx context.Context, namespace, id string, revision int, name string, spec Spec) (*Object, error) {
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
	item, err := lockedObject(ctx, tx, namespace, id, revision)
	if err != nil {
		return nil, err
	}
	previousTemplate := item.Spec.TemplateVersion
	item.Name, item.Spec = name, spec
	if err := item.NewObject.Validate(); err != nil {
		return nil, invalid(err.Error())
	}
	if err := validateReferences(ctx, tx, namespace, item.NewObject, false, previousTemplate); err != nil {
		return nil, err
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	item, err = scanObject(tx.QueryRow(ctx, "UPDATE pangu.catalog_objects SET name=$1,spec=$2,revision=revision+1,updated_at=now() WHERE id=$3 RETURNING "+objectColumns, name, data, id))
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

func Archive(ctx context.Context, namespace, id string, archived bool) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, "UPDATE pangu.catalog_objects SET archived=$1,updated_at=now() WHERE namespace=$2 AND id=$3", archived, namespace, id)
	if err == nil && result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func lockedObject(ctx context.Context, tx pgx.Tx, namespace, id string, revision int) (*Object, error) {
	item, err := scanObject(tx.QueryRow(ctx, "SELECT "+objectColumns+" FROM pangu.catalog_objects WHERE namespace=$1 AND id=$2 FOR UPDATE", namespace, id))
	if err != nil {
		return nil, err
	}
	if item.Archived {
		return nil, ErrArchived
	}
	if item.Revision != revision {
		return nil, ErrConflict
	}
	return item, nil
}

func scanObject(row pgx.Row) (*Object, error) {
	item := &Object{}
	var data []byte
	err := row.Scan(&item.ID, &item.Kind, &item.ProjectID, &item.Name, &item.Revision, &item.Archived, &data, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, json.Unmarshal(data, &item.Spec)
}
