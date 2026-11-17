package evolution

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s Service) rolloutPrepareTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, key, direction string) (*EntrySnapshot, error) {
	original, err := rolloutEntry(intent, key)
	if err != nil {
		return nil, err
	}
	if direction != "adopt" || intent.State != "switching" {
		return nil, ErrConflict
	}
	row, err := rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, key, direction)
	if err != nil {
		return nil, err
	}
	if row != nil {
		if row.Status == "conflict" {
			return nil, ErrConflict
		}
		return row.Result, nil
	}
	current, err := s.rolloutReadEntry(ctx, tx, scope, original)
	if err != nil {
		return nil, err
	}
	if !rolloutSameEntry(current, original) || current.RemoteState != "confirmed" || !current.Enabled || current.OwnerID == "" {
		return nil, ErrConflict
	}
	next := rolloutVersion(intent)
	if next == "" {
		return nil, ErrConflict
	}
	result, err := s.rolloutAdoptTx(ctx, tx, scope, intent, current, next)
	if err != nil {
		return nil, err
	}
	row = &rolloutSwitch{Expected: original, Result: &result, Status: result.RemoteState}
	if row.Status != "confirmed" && row.Status != "pending" && row.Status != "unknown" {
		return nil, ErrConflict
	}
	if err = rolloutSaveSwitch(ctx, tx, intent, key, direction, *row); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s Service) rolloutAdoptTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, current EntrySnapshot, next string) (EntrySnapshot, error) {
	_, err := tx.Exec(ctx, `INSERT INTO pangu.evolution_bindings(namespace,project_id,entry_key,adopted_version,stable_version) VALUES($1,$2,$3,$4,$4) ON CONFLICT DO NOTHING`, scope.Namespace, scope.ProjectID, current.Target.Key(), current.VersionID)
	if err != nil {
		return EntrySnapshot{}, err
	}
	b, err := rolloutBinding(ctx, tx, scope, current.Target.Key())
	if err != nil {
		return EntrySnapshot{}, err
	}
	if b.AdoptedVersion != current.VersionID || b.StableVersion != current.VersionID {
		return EntrySnapshot{}, ErrConflict
	}
	after, err := adoptBinding(b, b, intent.ID, intent.Generation, next)
	if err != nil {
		return EntrySnapshot{}, err
	}
	result := current
	result.VersionID = next
	result.Revision = after.Revision
	if current.Target.Kind != "workbench" {
		if s.Entries == nil {
			return EntrySnapshot{}, ErrForbidden
		}
		result, err = s.Entries.CompareAndSwapTx(ctx, tx, scope, rolloutEffectKey(intent, current.Target.Key(), "adopt"), current, next)
		if err != nil {
			return EntrySnapshot{}, err
		}
		if result.Target != current.Target || result.ProjectID != scope.ProjectID || result.OwnerID != current.OwnerID || !result.Enabled || result.VersionID != next || result.Revision <= current.Revision {
			return EntrySnapshot{}, ErrConflict
		}
	}
	if err = rolloutSaveBinding(ctx, tx, scope, b, after); err != nil {
		return EntrySnapshot{}, err
	}
	return result, nil
}

func (s Service) rolloutCompleteTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, key, direction string) error {
	row, err := rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, key, direction)
	if err != nil {
		return err
	}
	if row == nil || row.Result == nil {
		return ErrNotFound
	}
	if row.Status == "conflict" {
		return ErrConflict
	}
	current, err := s.rolloutReadEntry(ctx, tx, scope, *row.Result)
	if err != nil {
		return err
	}
	if !rolloutSameEntry(current, *row.Result) {
		row.Status = "conflict"
	} else {
		switch current.RemoteState {
		case "confirmed", "pending", "unknown":
			row.Status = current.RemoteState
		default:
			return ErrConflict
		}
	}
	if row.Status != "conflict" {
		row.Result = &current
	}
	return rolloutSaveSwitch(ctx, tx, intent, key, direction, *row)
}

func rolloutUnresolved(err error) bool {
	return errors.Is(err, ErrConflict) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound)
}
