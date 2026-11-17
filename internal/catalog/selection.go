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
	return RunnableChoicesForProject(ctx, namespace, "", offset)
}

func RunnableChoicesForProject(ctx context.Context, namespace, project string, offset int) ([]RunnableChoice, error) {
	if offset < 0 {
		return nil, invalid("offset 必须为非负整数")
	}
	return choices(ctx, namespace, project, false, offset)
}

func choices(ctx context.Context, namespace, project string, agentsOnly bool, offset int) ([]RunnableChoice, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT `+versionColumns+`,o.name FROM pangu.catalog_objects o
 JOIN LATERAL (SELECT cv.* FROM pangu.catalog_versions cv WHERE cv.object_id=o.id
 AND (cv.source_kind='manual' OR (cv.project_id=$2::uuid
 AND EXISTS(SELECT 1 FROM pangu.catalog_version_state s WHERE s.version_id=cv.id AND s.state='stable')
 AND EXISTS(SELECT 1 FROM pangu.catalog_version_bodies b WHERE b.version_id=cv.id AND b.spec IS NOT NULL)))
 ORDER BY cv.number DESC LIMIT 1) v ON true
 WHERE o.namespace=$1 AND NOT o.archived AND o.kind IN ('agent','workflow') AND (NOT $3 OR (o.kind='agent' AND o.merged_into IS NULL))
 ORDER BY o.name,o.id LIMIT CASE WHEN $3 THEN NULL ELSE 100 END OFFSET $4`, namespace, nullableID(project), agentsOnly, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RunnableChoice{}
	for rows.Next() {
		var item RunnableChoice
		var data, metadata []byte
		v := &item.Version
		if err := rows.Scan(&v.ID, &v.ObjectID, &v.Kind, &v.Number, &v.SourceRevision, &data, &v.CreatedAt, &v.Archived, &metadata, &item.Name); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metadata, v); err != nil {
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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Number   int    `json:"number"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

// AgentChoices 未指定项目时仅返回共享人工版本，不按列表页截断。
func AgentChoices(ctx context.Context, namespace string) ([]RunnableChoice, error) {
	return AgentChoicesForProject(ctx, namespace, "")
}

func AgentChoicesForProject(ctx context.Context, namespace, project string) ([]RunnableChoice, error) {
	return choices(ctx, namespace, project, true, 0)
}

func ConnectionChoices(ctx context.Context, namespace string, offset int) ([]ConnectionChoice, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT v.id::text,o.name,v.number,v.spec->'connection'->>'model',coalesce(nullif(v.spec->'connection'->>'provider',''),'openai')
 FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 WHERE o.namespace=$1 AND o.kind='connection' AND NOT o.archived AND v.source_kind='manual'
 ORDER BY v.created_at DESC,v.id DESC LIMIT 100 OFFSET $2`, namespace, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConnectionChoice{}
	for rows.Next() {
		var item ConnectionChoice
		if err = rows.Scan(&item.ID, &item.Name, &item.Number, &item.Model, &item.Provider); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
