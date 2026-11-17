package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
)

type evolutionPendingIntent struct {
	Scope      evolution.Scope
	ID         string
	Generation int
}

// ReconcileEvolution traverses every page; one stuck intent cannot starve later work.
func (b Backend) ReconcileEvolution(ctx context.Context) error {
	var failures []error
	cursor := ""
	for {
		items, err := evolutionPendingPage(ctx, b.Namespace(), cursor)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, item := range items {
			if ctx.Err() != nil {
				return errors.Join(append(failures, ctx.Err())...)
			}
			if err = b.DriveEvolution(ctx, item.Scope, item.ID, item.Generation); err != nil {
				failures = append(failures, err)
			}
			cursor = item.ID
		}
		if len(items) < 100 {
			break
		}
	}
	return errors.Join(failures...)
}

func evolutionPendingPage(ctx context.Context, namespace, cursor string) ([]evolutionPendingIntent, error) {
	conn, err := database.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	rows, err := conn.Query(ctx, `SELECT i.id::text,i.project_id::text,i.generation,p.actor_id FROM pangu.evolution_intents i JOIN pangu.experience_policies p ON p.namespace=i.namespace AND p.project_id=i.project_id AND p.revision=i.policy_revision WHERE i.namespace=$1 AND ($2='' OR i.id>NULLIF($2,'')::uuid) AND i.state IN ('evaluating','passed','switching','observing','rolling_back','reconciling') ORDER BY i.id LIMIT 100`, namespace, cursor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []evolutionPendingIntent{}
	for rows.Next() {
		item := evolutionPendingIntent{Scope: evolution.Scope{Namespace: namespace}}
		if err = rows.Scan(&item.ID, &item.Scope.ProjectID, &item.Generation, &item.Scope.ActorID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
