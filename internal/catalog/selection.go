package catalog

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
)

type RunnableChoice struct {
	Name    string  `json:"name"`
	Version Version `json:"version"`
}

func RunnableChoices(ctx context.Context, namespace string, offset int) ([]RunnableChoice, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT `+versionColumns+`,o.name FROM pangu.catalog_objects o
 JOIN LATERAL (SELECT * FROM pangu.catalog_versions WHERE object_id=o.id ORDER BY number DESC LIMIT 1) v ON true
 WHERE o.namespace=$1 AND NOT o.archived AND o.kind IN ('agent','workflow')
 ORDER BY o.name,o.id LIMIT 100 OFFSET $2`, namespace, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RunnableChoice{}
	for rows.Next() {
		var item RunnableChoice
		var data []byte
		v := &item.Version
		if err := rows.Scan(&v.ID, &v.ObjectID, &v.Kind, &v.Number, &v.SourceRevision, &data, &v.CreatedAt, &v.Archived, &item.Name); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &v.Spec); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type ConnectionChoice struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number int    `json:"number"`
	Model  string `json:"model"`
}

func ConnectionChoices(ctx context.Context, namespace string, offset int) ([]ConnectionChoice, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT v.id::text,o.name,v.number,v.spec->'connection'->>'model'
 FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 WHERE o.namespace=$1 AND o.kind='connection' AND NOT o.archived
 ORDER BY v.created_at DESC,v.id DESC LIMIT 100 OFFSET $2`, namespace, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConnectionChoice{}
	for rows.Next() {
		var item ConnectionChoice
		if err = rows.Scan(&item.ID, &item.Name, &item.Number, &item.Model); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
