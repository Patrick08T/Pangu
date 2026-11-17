package catalog

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type AutomaticStateGuard func(context.Context, pgx.Tx, Version) error

func validateAutomaticTransition(v Version, next string) error {
	if v.SourceKind != "auto" || v.Tombstoned {
		return ErrConflict
	}
	if v.State == "observing" && (next == "stable" || next == "revoked") {
		return nil
	}
	if v.State == "stable" && next == "revoked" {
		return nil
	}
	return ErrConflict
}

func readAutomatic(ctx context.Context, reader rowReader, namespace, project, id string) (*Version, error) {
	return scanVersion(reader.QueryRow(ctx, `SELECT `+versionColumns+` FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 WHERE o.namespace=$1 AND v.project_id=$2 AND v.id=$3 AND v.source_kind='auto'`, namespace, project, id))
}

// TransitionAutomaticTx 由同事务的策略/晋级意图守卫授权，普通HTTP不能设置原始状态。
func TransitionAutomaticTx(ctx context.Context, tx pgx.Tx, namespace, project, id string, expectedRevision int, next string, guard AutomaticStateGuard) error {
	if guard == nil {
		return invalid("生命周期缺少事务授权守卫")
	}
	v, err := readAutomatic(ctx, tx, namespace, project, id)
	if err != nil {
		return err
	}
	if err = guard(ctx, tx, *v); err != nil {
		return err
	}
	if err = validateAutomaticTransition(*v, next); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.catalog_version_state SET state=$1,revision=revision+1,updated_at=now()
 WHERE version_id=$2 AND revision=$3 AND state=$4`, next, id, expectedRevision, v.State)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

// ClearAutomaticVersionTx 墓碑保留版本身份，重复清理幂等且不能恢复正文。
func ClearAutomaticVersionTx(ctx context.Context, tx pgx.Tx, namespace, project, id string, guard AutomaticStateGuard) error {
	if guard == nil {
		return invalid("正文清理缺少事务授权守卫")
	}
	v, err := readAutomatic(ctx, tx, namespace, project, id)
	if err != nil {
		return err
	}
	if err = guard(ctx, tx, *v); err != nil {
		return err
	}
	if v.State != "revoked" {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.catalog_version_bodies SET spec=NULL,cleared_at=now() WHERE version_id=$1 AND spec IS NOT NULL`, id)
	return err
}

func ClearExperimentCandidateTx(ctx context.Context, tx pgx.Tx, namespace, project, id string, guard func(context.Context, pgx.Tx) error) error {
	if guard == nil {
		return invalid("候选清理缺少事务授权守卫")
	}
	if err := guard(ctx, tx); err != nil {
		return err
	}
	var invalidated bool
	err := tx.QueryRow(ctx, `SELECT status='invalidated' FROM pangu.evaluation_experiments WHERE namespace=$1 AND project_id=$2 AND id=$3 FOR UPDATE`, namespace, project, id).Scan(&invalidated)
	if err != nil {
		return err
	}
	if !invalidated {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.catalog_experiment_candidates SET payload=NULL,cleared_at=now() WHERE experiment_id=$1 AND payload IS NOT NULL`, id)
	return err
}
