package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func rolloutRecoveryTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int) (*intentRecord, experience.Policy, error) {
	var policy experience.Policy
	if err := lockScope(ctx, tx, scope); err != nil {
		return nil, policy, err
	}
	if scope.ActorID != "" {
		if err := managerTx(ctx, tx, scope); err != nil {
			return nil, policy, err
		}
	}
	intent, err := readIntentTx(ctx, tx, scope, id)
	if err != nil {
		return nil, policy, err
	}
	if intent.Generation != generation {
		return nil, policy, ErrConflict
	}
	var data []byte
	var actor string
	err = tx.QueryRow(ctx, `SELECT policy,actor_id::text FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=$3`, scope.Namespace, scope.ProjectID, intent.PolicyRevision).Scan(&data, &actor)
	if err != nil {
		return nil, policy, err
	}
	if err = json.Unmarshal(data, &policy); err != nil {
		return nil, policy, err
	}
	if policy.ActorID != actor || policy.ProjectID != scope.ProjectID || actor == "" {
		return nil, policy, ErrForbidden
	}
	if !slices.Contains([]string{"stable", "rolled_back", "insufficient"}, intent.State) {
		if err = verifyClaimsTx(ctx, tx, scope, intent); err != nil {
			return nil, policy, err
		}
	}
	return intent, policy, nil
}

func (s Service) rolloutRequestRollback(ctx context.Context, scope Scope, id string, generation int, reason string) (*Intent, error) {
	if strings.TrimSpace(reason) == "" || len(reason) > 1024 {
		return nil, ErrInvalid
	}
	var out *Intent
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, policy, err := rolloutRecoveryTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		scope.ActorID = policy.ActorID
		if err = rolloutBeginRollback(ctx, tx, scope, intent, reason, "rolled_back"); err != nil {
			return err
		}
		out = &intent.Intent
		return nil
	})
	return out, err
}

func rolloutBeginRollback(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, reason, terminal string) error {
	if intent.Publication == nil && !slices.Contains([]string{"rolled_back", "insufficient"}, intent.State) {
		return ErrConflict
	}
	if slices.Contains([]string{"rolled_back", "insufficient", "rolling_back", "reconciling"}, intent.State) {
		return nil
	}
	if !slices.Contains([]string{"switching", "observing", "stabilizing"}, intent.State) {
		return ErrConflict
	}
	if err := rolloutVersionsTx(ctx, tx, scope, intent, "revoked"); err != nil {
		return err
	}
	metadata := map[string]string{"reason": reason, "terminal": terminal}
	if _, err := tx.Exec(ctx, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{_rollout}',$2) WHERE id=$1`, intent.ID, jsonBytes(metadata)); err != nil {
		return err
	}
	return setIntentStateTx(ctx, tx, intent, "rolling_back")
}

func (s Service) rolloutRollbackTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, key string) (*EntrySnapshot, error) {
	original, err := rolloutEntry(intent, key)
	if err != nil {
		return nil, err
	}
	prior, err := rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, key, "rollback")
	if err != nil {
		return nil, err
	}
	if prior != nil {
		if prior.Status == "conflict" {
			return nil, ErrConflict
		}
		return prior.Result, nil
	}
	adopted, err := rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, key, "adopt")
	if err != nil {
		return nil, err
	}
	expected := original
	if adopted != nil {
		if adopted.Result == nil {
			return nil, ErrConflict
		}
		expected = *adopted.Result
	}
	current, err := s.rolloutReadEntry(ctx, tx, scope, expected)
	if err != nil {
		return nil, err
	}
	row := rolloutSwitch{Expected: expected, Result: &current, Status: current.RemoteState}
	if !rolloutSameEntry(current, expected) {
		row.Status = "conflict"
	} else if current.RemoteState == "confirmed" {
		if adopted != nil {
			current, err = s.rolloutRestoreTx(ctx, tx, scope, intent, current, original.VersionID)
			if err != nil {
				return nil, err
			}
			row.Result = &current
			row.Status = current.RemoteState
		}
	}
	if !slices.Contains([]string{"confirmed", "pending", "unknown", "conflict"}, row.Status) {
		return nil, ErrConflict
	}
	// 尚未开始补偿的远端未知状态不写完成快照；下轮必须先读取，不能盲目CAS。
	if adopted != nil && current.VersionID != original.VersionID && row.Status != "conflict" {
		return &current, nil
	}
	if err = rolloutSaveSwitch(ctx, tx, intent, key, "rollback", row); err != nil {
		return nil, err
	}
	return &current, nil
}

func (s Service) rolloutRestoreTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, current EntrySnapshot, baseline string) (EntrySnapshot, error) {
	b, err := rolloutBinding(ctx, tx, scope, current.Target.Key())
	if err != nil {
		return EntrySnapshot{}, err
	}
	if b.IntentID != intent.ID || b.Generation != intent.Generation || b.StableVersion != baseline || b.AdoptedVersion != current.VersionID {
		return EntrySnapshot{}, ErrConflict
	}
	if err = rolloutUsableVersion(ctx, tx, scope, baseline, false); err != nil {
		return EntrySnapshot{}, err
	}
	result := current
	result.VersionID = baseline
	result.Revision++
	if current.Target.Kind != "workbench" {
		if s.Entries == nil {
			return EntrySnapshot{}, ErrForbidden
		}
		result, err = s.Entries.CompareAndSwapTx(ctx, tx, scope, rolloutEffectKey(intent, current.Target.Key(), "rollback"), current, baseline)
		if err != nil {
			return EntrySnapshot{}, err
		}
		if result.Target != current.Target || result.ProjectID != scope.ProjectID || result.OwnerID != current.OwnerID || !result.Enabled || result.VersionID != baseline || result.Revision <= current.Revision {
			return EntrySnapshot{}, ErrConflict
		}
	}
	after := b
	after.AdoptedVersion = baseline
	after.Revision++
	if err = rolloutSaveBinding(ctx, tx, scope, b, after); err != nil {
		return EntrySnapshot{}, err
	}
	return result, nil
}

func (s Service) rolloutReconcileRollback(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	confirmed := true
	for _, entry := range intent.Config.Entries {
		_, err := s.rolloutRollbackTx(ctx, tx, scope, intent, entry.Target.Key())
		if rolloutUnresolved(err) {
			confirmed = false
			continue
		}
		if err != nil {
			return err
		}
		row, err := rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, entry.Target.Key(), "rollback")
		if err != nil {
			return err
		}
		if row == nil {
			confirmed = false
			continue
		}
		if err = s.rolloutCompleteTx(ctx, tx, scope, intent, entry.Target.Key(), "rollback"); rolloutUnresolved(err) {
			confirmed = false
			continue
		} else if err != nil {
			return err
		}
		row, err = rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, entry.Target.Key(), "rollback")
		if err != nil {
			return err
		}
		if row.Status != "confirmed" {
			confirmed = false
		}
	}
	if !confirmed {
		if intent.State != "reconciling" {
			return setIntentStateTx(ctx, tx, intent, "reconciling")
		}
		return nil
	}
	return rolloutFinishTx(ctx, tx, scope, intent, false)
}

func rolloutRecoverable(err error) bool {
	return errors.Is(err, ErrForbidden) || errors.Is(err, ErrConflict) || errors.Is(err, experience.ErrForbidden) || errors.Is(err, experience.ErrNotFound) || errors.Is(err, experience.ErrInvalidated) || errors.Is(err, experience.ErrConflict)
}
