package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
)

func rolloutEffectKey(intent *intentRecord, key, direction string) string {
	return intent.ID + ":" + strconv.Itoa(intent.Generation) + ":" + key + ":" + direction
}

type rolloutSwitch struct {
	Expected EntrySnapshot
	Result   *EntrySnapshot
	Status   string
}

func rolloutEntry(intent *intentRecord, key string) (EntrySnapshot, error) {
	for _, entry := range intent.Config.Entries {
		if entry.Target.Key() == key {
			return entry, nil
		}
	}
	return EntrySnapshot{}, ErrForbidden
}

func rolloutVersion(intent *intentRecord) string {
	if intent.Publication == nil {
		return ""
	}
	if intent.Publication.FlowID != "" {
		return intent.Publication.FlowID
	}
	return intent.Publication.AgentID
}

func rolloutReadSwitch(ctx context.Context, tx pgx.Tx, id string, generation int, key, direction string) (*rolloutSwitch, error) {
	var row rolloutSwitch
	var expected, result []byte
	err := tx.QueryRow(ctx, `SELECT expected,result,status FROM pangu.evolution_switches WHERE intent_id=$1 AND generation=$2 AND entry_key=$3 AND direction=$4 FOR UPDATE`, id, generation, key, direction).Scan(&expected, &result, &row.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(expected, &row.Expected); err != nil {
		return nil, err
	}
	if len(result) > 0 {
		if err = json.Unmarshal(result, &row.Result); err != nil {
			return nil, err
		}
	}
	return &row, nil
}

func rolloutSaveSwitch(ctx context.Context, tx pgx.Tx, intent *intentRecord, key, direction string, row rolloutSwitch) error {
	var result any
	if row.Result != nil {
		result = jsonBytes(row.Result)
	}
	_, err := tx.Exec(ctx, `INSERT INTO pangu.evolution_switches(intent_id,generation,entry_key,direction,expected,result,status) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(intent_id,generation,entry_key,direction) DO UPDATE SET result=excluded.result,status=excluded.status,updated_at=clock_timestamp()`, intent.ID, intent.Generation, key, direction, jsonBytes(row.Expected), result, row.Status)
	return err
}

func rolloutBinding(ctx context.Context, tx pgx.Tx, scope Scope, key string) (Binding, error) {
	var b Binding
	err := tx.QueryRow(ctx, `SELECT entry_key,revision,adopted_version::text,stable_version::text,coalesce(intent_id::text,''),generation FROM pangu.evolution_bindings WHERE namespace=$1 AND project_id=$2 AND entry_key=$3 FOR UPDATE`, scope.Namespace, scope.ProjectID, key).Scan(&b.EntryKey, &b.Revision, &b.AdoptedVersion, &b.StableVersion, &b.IntentID, &b.Generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

func rolloutSaveBinding(ctx context.Context, tx pgx.Tx, scope Scope, before, after Binding) error {
	tag, err := tx.Exec(ctx, `UPDATE pangu.evolution_bindings SET revision=$4,adopted_version=$5,stable_version=$6,intent_id=nullif($7,'')::uuid,generation=$8 WHERE namespace=$1 AND project_id=$2 AND entry_key=$3 AND revision=$9 AND adopted_version=$10 AND stable_version=$11 AND coalesce(intent_id::text,'')=$12 AND generation=$13`, scope.Namespace, scope.ProjectID, before.EntryKey, after.Revision, after.AdoptedVersion, after.StableVersion, after.IntentID, after.Generation, before.Revision, before.AdoptedVersion, before.StableVersion, before.IntentID, before.Generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (s Service) rolloutReadEntry(ctx context.Context, tx pgx.Tx, scope Scope, expected EntrySnapshot) (EntrySnapshot, error) {
	if expected.Target.Kind != "workbench" {
		if s.Entries == nil {
			return EntrySnapshot{}, ErrForbidden
		}
		return s.Entries.ReadTx(ctx, tx, scope, expected.Target)
	}
	b, err := rolloutBinding(ctx, tx, scope, expected.Target.Key())
	if err != nil {
		return EntrySnapshot{}, err
	}
	expected.VersionID = b.AdoptedVersion
	expected.Revision = b.Revision
	expected.RemoteState = "confirmed"
	return expected, nil
}

func rolloutSameEntry(a, b EntrySnapshot) bool {
	return a.Target == b.Target && a.ProjectID == b.ProjectID && a.OwnerID == b.OwnerID && a.Enabled == b.Enabled && a.VersionID == b.VersionID && a.Revision == b.Revision
}
