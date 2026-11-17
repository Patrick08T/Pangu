package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func (b Backend) experienceErasureStop(ctx context.Context, item experienceErasureBinding) error {
	// GetRun consults live execution first; only its explicit not-found path may
	// use a trustworthy archived receipt. An old receipt cannot mask a live run.
	run, err := b.GetRun(ctx, item.run)
	if err != nil {
		return errors.Join(ErrExperienceErasureDeferred, err)
	}
	if run.ID != item.run || run.ProjectID != item.project || !run.StopConfirmed || !terminal(run.Status) || run.FinishedAt == nil {
		_, cancelErr := b.cancelTree(ctx, item.run)
		return errors.Join(ErrExperienceErasureDeferred, cancelErr)
	}
	for _, node := range run.Nodes {
		if !terminal(node.Status) {
			return ErrExperienceErasureDeferred
		}
	}
	active, err := execution.Active(ctx, item.run)
	if err != nil {
		return err
	}
	if active != 0 {
		return ErrExperienceErasureDeferred
	}
	return nil
}

func experienceErasureStoppedTx(ctx context.Context, tx pgx.Tx, item experienceErasureBinding) error {
	var safe bool
	err := tx.QueryRow(ctx, `SELECT coalesce(s.snapshot->>'purpose','') IN ('','business')
 AND EXISTS(SELECT 1 FROM pangu.run_cancellations c WHERE c.run_id=s.run_id AND c.namespace=s.namespace)
 AND NOT EXISTS(SELECT 1 FROM pangu.node_attempts n WHERE n.run_id=s.run_id AND n.finished_at IS NULL)
 AND EXISTS(SELECT 1 FROM pangu.run_history h WHERE h.namespace=s.namespace AND h.run_id=s.run_id
 AND h.finished_at<=clock_timestamp() AND h.summary->>'id'=s.run_id::text AND h.summary->>'project_id'=s.project_id::text
 AND h.summary->>'status' IN ('succeeded','failed','cancelled','skipped') AND h.summary->'stop_confirmed'='true'::jsonb
 AND h.summary->>'finished_at' IS NOT NULL AND jsonb_typeof(h.summary->'nodes')='array'
 AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(h.summary->'nodes')='array' THEN h.summary->'nodes' ELSE '[{}]'::jsonb END) child WHERE coalesce(child->>'status','') NOT IN ('succeeded','failed','cancelled','skipped')))
 FROM pangu.submissions s WHERE s.namespace=$1 AND s.project_id=$2 AND s.id=$3 AND s.run_id=$4`, item.namespace, item.project, item.request, item.run).Scan(&safe)
	if err == pgx.ErrNoRows {
		return ErrExperienceErasureDeferred
	}
	if err != nil {
		return err
	}
	if !safe {
		return ErrExperienceErasureDeferred
	}
	return nil
}

func experienceErasureGuard(ctx context.Context, tx pgx.Tx, root, target experienceErasureBinding, expected []experience.Reference) error {
	refs, err := experienceErasureProofTx(ctx, tx, root)
	if err != nil {
		return err
	}
	if len(refs) == 0 || experienceErasureDigest(refs) != experienceErasureDigest(expected) {
		return ErrExperienceErasureDeferred
	}
	children, err := evolutionConsumerClosureTx(ctx, tx, evolution.Scope{Namespace: root.namespace, ProjectID: root.project}, evolutionConsumer{request: root.request, run: root.run})
	if err != nil {
		return err
	}
	found := false
	for _, child := range children {
		if child.run == "" || child.evaluation {
			return ErrExperienceErasureDeferred
		}
		item := experienceErasureBinding{root.namespace, root.project, child.request, child.run}
		if item == target {
			found = true
		}
		if err = experienceErasureStoppedTx(ctx, tx, item); err != nil {
			return err
		}
	}
	if !found {
		return ErrExperienceErasureDeferred
	}
	return nil
}
