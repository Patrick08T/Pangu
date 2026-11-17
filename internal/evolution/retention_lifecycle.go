package evolution

import (
	"context"
	"slices"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/jackc/pgx/v5"
)

func retentionRevokeTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, target RetentionTarget, reason string) error {
	for _, id := range target.VersionIDs {
		var state string
		var revision int
		err := tx.QueryRow(ctx, `SELECT s.state,s.revision FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id JOIN pangu.catalog_version_state s ON s.version_id=v.id JOIN pangu.catalog_version_bodies b ON b.version_id=v.id WHERE o.namespace=$1 AND v.project_id=$2 AND v.id=$3 AND v.source_kind='auto' AND v.source_experiment=$4 FOR UPDATE OF s,b FOR SHARE OF v,o`, scope.Namespace, scope.ProjectID, id, target.ExperimentID).Scan(&state, &revision)
		if err != nil {
			return err
		}
		if state != "revoked" {
			if err = catalog.TransitionAutomaticTx(ctx, tx, scope.Namespace, scope.ProjectID, id, revision, "revoked", retentionVersionGuard(scope, target)); err != nil {
				return err
			}
		}
	}
	if intent.Publication == nil && slices.Contains([]string{"preparing", "evaluating", "passed"}, intent.State) {
		if err := verifyClaimsTx(ctx, tx, scope, intent); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{_cancel}',$2) WHERE id=$1`, intent.ID, jsonBytes(map[string]string{"reason": reason})); err != nil {
			return err
		}
		return setIntentStateTx(ctx, tx, intent, "reconciling")
	}
	return nil
}

func retentionVersionGuard(scope Scope, target RetentionTarget) catalog.AutomaticStateGuard {
	return func(_ context.Context, _ pgx.Tx, v catalog.Version) error {
		if v.SourceKind != "auto" || v.ProjectID != scope.ProjectID || v.SourceExperiment != target.ExperimentID || !slices.Contains(target.VersionIDs, v.ID) {
			return ErrForbidden
		}
		return nil
	}
}

func (s Service) retentionSettleTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, reason string) (bool, error) {
	if slices.Contains([]string{"stable", "rolled_back", "insufficient", "rejected", "invalidated"}, intent.State) {
		return true, nil
	}
	if err := verifyClaimsTx(ctx, tx, scope, intent); err != nil {
		return false, err
	}
	if intent.Publication == nil {
		if err := s.cancelEvaluationTx(ctx, tx, scope, intent, reason); err != nil {
			return false, err
		}
		return intent.State == "insufficient", nil
	}
	if err := rolloutBeginRollback(ctx, tx, scope, intent, reason, "insufficient"); err != nil {
		return false, err
	}
	if err := s.rolloutReconcileRollback(ctx, tx, scope, intent); err != nil {
		return false, err
	}
	return intent.State == "rolled_back" || intent.State == "insufficient", nil
}

func retentionUnreferencedTx(ctx context.Context, tx pgx.Tx, scope Scope, target RetentionTarget, guard RetentionGuard) (bool, error) {
	ready, err := retentionCheck(ctx, tx, scope, target, "unreferenced", guard)
	if err != nil || !ready {
		return false, err
	}
	var referenced bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_bindings WHERE namespace=$1 AND project_id=$2 AND (adopted_version::text=ANY($3::text[]) OR stable_version::text=ANY($3::text[])))`, scope.Namespace, scope.ProjectID, target.VersionIDs).Scan(&referenced)
	return !referenced, err
}
