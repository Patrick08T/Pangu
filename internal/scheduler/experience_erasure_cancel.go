package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

func experienceErasureCancelTx(ctx context.Context, tx pgx.Tx, root experienceErasureBinding) ([]experienceErasureBinding, []experience.Reference, error) {
	if err := identity.Lock(ctx, tx, root.namespace); err != nil {
		return nil, nil, err
	}
	if err := experience.Lock(ctx, tx, root.namespace, root.project); err != nil {
		return nil, nil, err
	}
	refs, err := experienceErasureProofTx(ctx, tx, root)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}
	if root.run == "" {
		return nil, nil, ErrExperienceErasureDeferred
	}
	children, err := evolutionConsumerClosureTx(ctx, tx, evolution.Scope{Namespace: root.namespace, ProjectID: root.project}, evolutionConsumer{request: root.request, run: root.run})
	if err != nil {
		return nil, nil, err
	}
	out := make([]experienceErasureBinding, 0, len(children))
	for _, child := range children {
		if child.run == "" || child.evaluation {
			return nil, nil, ErrExperienceErasureDeferred
		}
		out = append(out, experienceErasureBinding{root.namespace, root.project, child.request, child.run})
	}
	for _, child := range out {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, child.run); err != nil {
			return nil, nil, err
		}
	}
	for _, child := range out {
		_, err = tx.Exec(ctx, `INSERT INTO pangu.run_cancellations(run_id,namespace) VALUES($1,$2) ON CONFLICT(run_id) DO NOTHING`, child.run, child.namespace)
		if err != nil {
			return nil, nil, err
		}
	}
	if err = experienceErasureEventTx(ctx, tx, root, "Cancelled", refs); err != nil {
		return nil, nil, err
	}
	return out, refs, nil
}

func experienceErasureEventTx(ctx context.Context, tx pgx.Tx, root experienceErasureBinding, phase string, refs []experience.Reference) error {
	// Only fixed phases and immutable IDs/hashes; never retain callback errors or bodies.
	_, err := tx.Exec(ctx, `INSERT INTO pangu.experience_events(namespace,project_id,actor_id,action,subject_id,detail)
 SELECT $1,$2,'server:retention','derived_copy_erasure',$3,jsonb_build_object('phase',$4::text,'request_id',$5::text,'roots',$6::jsonb)
 WHERE NOT EXISTS(SELECT 1 FROM pangu.experience_events WHERE namespace=$1 AND project_id=$2 AND action='derived_copy_erasure' AND subject_id=$3 AND detail->>'phase'=$4)`, root.namespace, root.project, root.run, phase, root.request, refs)
	return err
}
