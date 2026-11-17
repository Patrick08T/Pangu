package evolution

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/jackc/pgx/v5"
)

func retentionClearTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, target RetentionTarget, out *RetentionResult) error {
	if err := retentionInvalidateTx(ctx, tx, scope, intent); err != nil {
		return err
	}
	if target.ExperimentID != "" {
		var present bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.catalog_experiment_candidates WHERE experiment_id=$1 AND payload IS NOT NULL)`, target.ExperimentID).Scan(&present)
		if err != nil {
			return err
		}
		if present {
			guard := func(ctx context.Context, tx pgx.Tx) error { return retentionInvalidateTx(ctx, tx, scope, intent) }
			if err = catalog.ClearExperimentCandidateTx(ctx, tx, scope.Namespace, scope.ProjectID, target.ExperimentID, guard); err != nil {
				return err
			}
			out.CandidateCleared = true
		}
	}
	for _, id := range target.VersionIDs {
		var present bool
		if err := tx.QueryRow(ctx, `SELECT spec IS NOT NULL FROM pangu.catalog_version_bodies WHERE version_id=$1`, id).Scan(&present); err != nil {
			return err
		}
		if !present {
			continue
		}
		if err := catalog.ClearAutomaticVersionTx(ctx, tx, scope.Namespace, scope.ProjectID, id, retentionVersionGuard(scope, target)); err != nil {
			return err
		}
		out.VersionsCleared++
	}
	_, err := tx.Exec(ctx, `UPDATE pangu.evolution_intents SET request=jsonb_set(jsonb_set(request,'{request,payload,agent}','{}'::jsonb),'{_retention}',jsonb_build_object('reason',$2::text,'erased_at',coalesce(request#>>'{_retention,erased_at}',clock_timestamp()::text))) WHERE id=$1`, intent.ID, out.Reason)
	return err
}

func retentionCandidatePresentTx(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	var present bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.catalog_experiment_candidates WHERE experiment_id=$1 AND payload IS NOT NULL)`, id).Scan(&present)
	return present, err
}

func retentionInvalidateTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	if intent.ExperimentID == "" {
		return nil
	}
	var status, hash string
	var generation int
	err := tx.QueryRow(ctx, `SELECT status,generation,spec_hash FROM pangu.evaluation_experiments WHERE namespace=$1 AND project_id=$2 AND id=$3 FOR UPDATE`, scope.Namespace, scope.ProjectID, intent.ExperimentID).Scan(&status, &generation, &hash)
	if err != nil {
		return err
	}
	if hash != intent.ExperimentHash {
		return ErrConflict
	}
	if status == "invalidated" {
		return nil
	}
	if generation != intent.Generation {
		return ErrConflict
	}
	return evaluation.InvalidateExperimentTx(ctx, tx, scope.Namespace, intent.ExperimentID, generation)
}
