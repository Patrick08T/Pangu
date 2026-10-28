package catalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const objectColumns = `id::text,kind,name,revision,archived,spec,created_at`

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
	item, err := createObject(ctx, tx, namespace, input)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

func createObject(ctx context.Context, tx pgx.Tx, namespace string, input NewObject) (*Object, error) {
	if err := input.Validate(); err != nil {
		return nil, invalid(err.Error())
	}
	if err := validateReferences(ctx, tx, namespace, input, false); err != nil {
		return nil, err
	}
	data, err := json.Marshal(input.Spec)
	if err != nil {
		return nil, err
	}
	item, err := scanObject(tx.QueryRow(ctx, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1,$2,$3,$4,$5) RETURNING `+objectColumns, uuid.NewString(), namespace, input.Kind, input.Name, data))
	if err != nil {
		return nil, err
	}
	return item, nil
}

func Get(ctx context.Context, namespace, id string) (*Object, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scanObject(conn.QueryRow(ctx, "SELECT "+objectColumns+" FROM pangu.catalog_objects WHERE namespace=$1 AND id=$2", namespace, id))
}

func List(ctx context.Context, namespace, kind string, offset int) ([]Object, error) {
	if offset < 0 {
		return nil, errors.New("offset 必须为非负整数")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+objectColumns+" FROM pangu.catalog_objects WHERE namespace=$1 AND merged_into IS NULL AND ($2='' OR kind=$2) ORDER BY created_at,id OFFSET $3 LIMIT 100", namespace, kind, offset)
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
	item, err := saveObject(ctx, tx, namespace, id, revision, name, spec)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

func saveObject(ctx context.Context, tx pgx.Tx, namespace, id string, revision int, name string, spec Spec) (*Object, error) {
	item, err := lockedObject(ctx, tx, namespace, id, revision)
	if err != nil {
		return nil, err
	}
	item.Name, item.Spec = name, spec
	if err := item.NewObject.Validate(); err != nil {
		return nil, invalid(err.Error())
	}
	if err := validateReferences(ctx, tx, namespace, item.NewObject, false); err != nil {
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
	return item, nil
}

func Archive(ctx context.Context, namespace, id string, archived bool) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, "UPDATE pangu.catalog_objects SET archived=$1,updated_at=now() WHERE namespace=$2 AND id=$3 AND merged_into IS NULL", archived, namespace, id)
	if err == nil && result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func lockedObject(ctx context.Context, tx pgx.Tx, namespace, id string, revision int) (*Object, error) {
	item, err := scanObject(tx.QueryRow(ctx, "SELECT "+objectColumns+" FROM pangu.catalog_objects WHERE namespace=$1 AND id=$2 AND merged_into IS NULL FOR UPDATE", namespace, id))
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
	err := row.Scan(&item.ID, &item.Kind, &item.Name, &item.Revision, &item.Archived, &data, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, json.Unmarshal(data, &item.Spec)
}
