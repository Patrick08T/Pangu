package evolution

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/jackc/pgx/v5"
)

func rolloutUsableVersion(ctx context.Context, tx pgx.Tx, scope Scope, id string, observing bool) error {
	var kind string
	err := tx.QueryRow(ctx, `SELECT v.source_kind FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE v.id=$1 AND o.namespace=$2 AND NOT o.archived AND o.merged_into IS NULL AND (v.source_kind='manual' OR v.project_id=$3) FOR SHARE OF o`, id, scope.Namespace, scope.ProjectID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if kind == "manual" {
		return nil
	}
	var state string
	var body bool
	err = tx.QueryRow(ctx, `SELECT s.state,b.spec IS NOT NULL FROM pangu.catalog_version_state s JOIN pangu.catalog_version_bodies b ON b.version_id=s.version_id WHERE s.version_id=$1 FOR SHARE OF s,b`, id).Scan(&state, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if !body || (state != "stable" && !(observing && state == "observing")) {
		return ErrConflict
	}
	return nil
}

func rolloutPublicationUsable(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	if intent.Publication == nil || intent.Publication.AgentID == "" {
		return ErrConflict
	}
	ids := []string{intent.Publication.AgentID}
	if intent.Publication.FlowID != "" {
		ids = append(ids, intent.Publication.FlowID)
	}
	for _, id := range ids {
		if err := rolloutUsableVersion(ctx, tx, scope, id, true); err != nil {
			return err
		}
	}
	return nil
}

func rolloutVersionsTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, state string) error {
	if intent.Publication == nil || intent.Publication.AgentID == "" {
		return ErrConflict
	}
	ids := []string{intent.Publication.AgentID}
	if intent.Publication.FlowID != "" {
		ids = append(ids, intent.Publication.FlowID)
	}
	for _, id := range ids {
		var revision int
		var current string
		err := tx.QueryRow(ctx, `SELECT s.revision,s.state FROM pangu.catalog_version_state s JOIN pangu.catalog_versions v ON v.id=s.version_id JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE v.id=$1 AND o.namespace=$2 AND v.project_id=$3 FOR UPDATE OF s`, id, scope.Namespace, scope.ProjectID).Scan(&revision, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if current == state {
			continue
		}
		guard := func(ctx context.Context, tx pgx.Tx, v catalog.Version) error {
			if v.ID != id || v.ProjectID != scope.ProjectID || v.SourceKind != "auto" {
				return ErrForbidden
			}
			return verifyClaimsTx(ctx, tx, scope, intent)
		}
		if err = catalog.TransitionAutomaticTx(ctx, tx, scope.Namespace, scope.ProjectID, id, revision, state, guard); err != nil {
			return err
		}
	}
	return nil
}

func rolloutFinishTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, stable bool) error {
	if !stable {
		if held, err := retentionHoldRollbackTx(ctx, tx, intent); err != nil || held {
			return err
		}
	}
	for _, entry := range intent.Config.Entries {
		before, err := rolloutBinding(ctx, tx, scope, entry.Target.Key())
		if err != nil {
			return err
		}
		if before.IntentID == "" && !stable {
			if before.AdoptedVersion != entry.VersionID || before.StableVersion != entry.VersionID {
				return ErrConflict
			}
			continue
		}
		after, err := finishBinding(before, intent.ID, intent.Generation, stable)
		if err != nil {
			return err
		}
		if err = rolloutSaveBinding(ctx, tx, scope, before, after); err != nil {
			return err
		}
	}
	terminal := "stable"
	if stable {
		if err := rolloutVersionsTx(ctx, tx, scope, intent, "stable"); err != nil {
			return err
		}
	} else {
		if err := tx.QueryRow(ctx, `SELECT coalesce(request#>>'{_rollout,terminal}','rolled_back') FROM pangu.evolution_intents WHERE id=$1`, intent.ID).Scan(&terminal); err != nil {
			return err
		}
		if terminal != "rolled_back" && terminal != "insufficient" {
			return ErrConflict
		}
	}
	if err := setIntentStateTx(ctx, tx, intent, terminal); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM pangu.evolution_target_claims WHERE namespace=$1 AND project_id=$2 AND intent_id=$3 AND generation=$4`, scope.Namespace, scope.ProjectID, intent.ID, intent.Generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != int64(len(intent.Targets)) {
		return ErrConflict
	}
	return nil
}
