package workspace

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type experienceErasureScope struct{ namespace, project, request, run string }

func experienceErasureLease(ctx context.Context, s experienceErasureScope) (*pgx.Conn, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	// Try workspace/run leases: fork import can already own workspace while it
	// approaches identity authorization. Never invert that wait into a deadlock.
	for _, key := range []string{"identity:" + s.namespace, "experience:" + s.namespace + ":" + s.project, "workspace:" + s.run, "retention:" + s.run, s.run} {
		var locked bool
		err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&locked)
		if err != nil || !locked {
			_ = conn.Close(context.Background())
			if err == nil {
				err = ErrExperienceErasureDeferred
			}
			return nil, err
		}
	}
	return conn, nil
}

func experienceErasureVerifyTx(ctx context.Context, tx pgx.Tx, s experienceErasureScope, verify ExperienceErasureVerifier) error {
	if verify == nil {
		return ErrExperienceErasureDeferred
	}
	var safe bool
	err := tx.QueryRow(ctx, `SELECT coalesce(s.snapshot->>'purpose','') IN ('','business')
 AND EXISTS(SELECT 1 FROM pangu.run_cancellations c WHERE c.run_id=s.run_id AND c.namespace=s.namespace)
 AND NOT EXISTS(SELECT 1 FROM pangu.node_attempts n WHERE n.run_id=s.run_id AND n.finished_at IS NULL)
 AND EXISTS(SELECT 1 FROM pangu.run_history h WHERE h.namespace=s.namespace AND h.run_id=s.run_id
 AND h.finished_at<=clock_timestamp() AND h.summary->>'id'=s.run_id::text AND h.summary->>'project_id'=s.project_id::text
 AND h.summary->>'status' IN ('succeeded','failed','cancelled','skipped') AND h.summary->'stop_confirmed'='true'::jsonb
 AND h.summary->>'finished_at' IS NOT NULL AND jsonb_typeof(h.summary->'nodes')='array'
 AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(h.summary->'nodes')='array' THEN h.summary->'nodes' ELSE '[{}]'::jsonb END) child WHERE coalesce(child->>'status','') NOT IN ('succeeded','failed','cancelled','skipped')))
 FROM pangu.submissions s JOIN pangu.projects p ON p.id=s.project_id AND p.namespace=s.namespace
 WHERE s.namespace=$1 AND s.project_id=$2 AND s.id=$3 AND s.run_id=$4 FOR UPDATE OF s`, s.namespace, s.project, s.request, s.run).Scan(&safe)
	if err == pgx.ErrNoRows {
		return ErrExperienceErasureDeferred
	}
	if err != nil {
		return err
	}
	if !safe {
		return ErrExperienceErasureDeferred
	}
	return verify(ctx, tx)
}

func experienceErasureIntent(ctx context.Context, conn *pgx.Conn, s experienceErasureScope, verify ExperienceErasureVerifier) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = experienceErasureVerifyTx(ctx, tx, s, verify); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.workspace_runs SET expired_at=coalesce(expired_at,clock_timestamp()),finished_at=coalesce(finished_at,(SELECT finished_at FROM pangu.run_history WHERE run_id=$1)) WHERE run_id=$1 AND project_id=$2`, s.run, s.project)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
