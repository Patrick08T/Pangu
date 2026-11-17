package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

const evolutionConsumerClosureSQL = `WITH RECURSIVE related AS (
 SELECT id,run_id,namespace,project_id FROM pangu.submissions WHERE id=$1 AND run_id=$2
 UNION SELECT s.id,s.run_id,s.namespace,s.project_id FROM related r JOIN pangu.approvals a ON a.run_id=r.run_id
 JOIN pangu.plan_continuations c ON c.approval_id=a.id JOIN pangu.submissions s ON s.id=c.request_id)
 SELECT r.id::text,coalesce(r.run_id::text,''),r.namespace,r.project_id::text,e.request_id IS NOT NULL,coalesce(p.actor_id::text,'')
 FROM related r LEFT JOIN pangu.evolution_executions e ON e.request_id=r.id
 LEFT JOIN pangu.evolution_intents i ON i.id=e.intent_id AND i.namespace=r.namespace AND i.project_id=r.project_id
 LEFT JOIN pangu.experience_policies p ON p.namespace=i.namespace AND p.project_id=i.project_id AND p.revision=i.policy_revision
 ORDER BY r.id LIMIT 1025`

func evolutionConsumerClosureTx(ctx context.Context, tx pgx.Tx, scope evolution.Scope, root evolutionConsumer) ([]evolutionConsumer, error) {
	rows, err := tx.Query(ctx, evolutionConsumerClosureSQL, root.request, root.run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []evolutionConsumer
	for rows.Next() {
		var item evolutionConsumer
		var namespace, project string
		if err = rows.Scan(&item.request, &item.run, &namespace, &project, &item.evaluation, &item.actor); err != nil {
			return nil, err
		}
		if namespace != scope.Namespace || project != scope.ProjectID {
			return nil, evolution.ErrConflict
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, pgx.ErrNoRows
	}
	// An excessive or unverifiable graph is not permission to cancel a subset.
	if len(out) > 1024 {
		return nil, ErrEvolutionConsumerStopUnknown
	}
	return out, nil
}

func evolutionConsumerRequestCancel(ctx context.Context, scope evolution.Scope, item evolutionConsumer) error {
	return evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := identity.Lock(ctx, tx, scope.Namespace); err != nil {
			return err
		}
		if err := experience.Lock(ctx, tx, scope.Namespace, scope.ProjectID); err != nil {
			return err
		}
		closure, err := evolutionConsumerClosureTx(ctx, tx, scope, item)
		if err != nil {
			return err
		}
		for _, child := range closure {
			if child.run == "" {
				continue
			}
			if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, child.run); err != nil {
				return err
			}
		}
		// Same persistent execution barrier as RequestCancel, but only validated IDs;
		// the general recursive helper cannot enforce this project boundary.
		for _, child := range closure {
			if child.run == "" {
				continue
			}
			_, err = tx.Exec(ctx, `INSERT INTO pangu.run_cancellations(run_id,namespace) VALUES($1,$2) ON CONFLICT(run_id) DO NOTHING`, child.run, scope.Namespace)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
