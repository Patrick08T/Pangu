package scheduler

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type EvolutionEntries struct{}

func (EvolutionEntries) ReadTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, t evolution.EntryTarget) (evolution.EntrySnapshot, error) {
	out := evolution.EntrySnapshot{Target: t, RemoteState: "confirmed"}
	if _, err := uuid.Parse(t.ID); err != nil {
		return out, evolution.ErrInvalid
	}
	if err := identity.Lock(ctx, tx, s.Namespace); err != nil {
		return out, err
	}
	var err error
	switch t.Kind {
	case "cron":
		_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "schedule:"+t.ID)
		if err == nil {
			err = tx.QueryRow(ctx, `SELECT project_id::text,workflow->>'version_id',revision,coalesce(created_by::text,''),NOT(paused OR deleted),CASE WHEN remote_synced THEN 'confirmed' ELSE 'pending' END FROM pangu.schedules WHERE namespace=$1 AND id=$2 AND project_id=$3 FOR UPDATE`, s.Namespace, t.ID, s.ProjectID).Scan(&out.ProjectID, &out.VersionID, &out.Revision, &out.OwnerID, &out.Enabled, &out.RemoteState)
		}
	case "webhook":
		err = tx.QueryRow(ctx, `SELECT project_id::text,version_id::text,revision,actor_id::text,enabled FROM pangu.github_hooks WHERE namespace=$1 AND id=$2 AND project_id=$3 FOR UPDATE`, s.Namespace, t.ID, s.ProjectID).Scan(&out.ProjectID, &out.VersionID, &out.Revision, &out.OwnerID, &out.Enabled)
	default:
		return out, evolution.ErrInvalid
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, evolution.ErrNotFound
	}
	if err != nil {
		return out, err
	}
	allowed, err := identity.ScheduleOwner(ctx, tx, out.OwnerID, s.ProjectID)
	if err != nil {
		return out, err
	}
	if !allowed {
		return out, evolution.ErrForbidden
	}
	return out, nil
}

func (a EvolutionEntries) CompareAndSwapTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, key string, old evolution.EntrySnapshot, next string) (evolution.EntrySnapshot, error) {
	current, err := a.ReadTx(ctx, tx, s, old.Target)
	if err != nil {
		return current, err
	}
	if current != old || !current.Enabled || key == "" || next == old.VersionID {
		return current, evolution.ErrConflict
	}
	scope := catalog.VersionScope{ProjectID: s.ProjectID, AllowObserving: true}
	var object, kind string
	if err = tx.QueryRow(ctx, `SELECT o.id::text,o.kind FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE v.id=$1 AND o.namespace=$2`, old.VersionID, s.Namespace).Scan(&object, &kind); err != nil {
		return current, err
	}
	after, err := catalog.ReadVersionScopedTx(ctx, tx, s.Namespace, next, scope)
	if err != nil {
		return current, err
	}
	if after.Archived || object != after.ObjectID || kind != after.Kind {
		return current, evolution.ErrConflict
	}
	if old.Target.Kind == "cron" {
		return evolutionCronCASTx(ctx, tx, s, key, old, next)
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.github_hooks SET version_id=$1 WHERE namespace=$2 AND id=$3 AND project_id=$4 AND version_id=$5 AND revision=$6 AND actor_id=$7 AND enabled`, next, s.Namespace, old.Target.ID, s.ProjectID, old.VersionID, old.Revision, old.OwnerID)
	if err != nil {
		return current, err
	}
	if result.RowsAffected() != 1 {
		return current, evolution.ErrConflict
	}
	return a.ReadTx(ctx, tx, s, old.Target)
}

func evolutionCronCASTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, key string, old evolution.EntrySnapshot, next string) (evolution.EntrySnapshot, error) {
	var raw []byte
	var previous workflow.Definition
	if err := tx.QueryRow(ctx, `SELECT workflow FROM pangu.schedules WHERE id=$1 AND namespace=$2`, old.Target.ID, s.Namespace).Scan(&raw); err != nil {
		return old, err
	}
	if err := json.Unmarshal(raw, &previous); err != nil {
		return old, err
	}
	compiled, err := catalog.CompileScoped(ctx, s.Namespace, next, catalog.VersionScope{ProjectID: s.ProjectID, AllowObserving: true}, previous.Input)
	if err != nil {
		return old, err
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.schedules SET workflow=$1,revision=revision+1,remote_synced=false WHERE namespace=$2 AND id=$3 AND project_id=$4 AND workflow->>'version_id'=$5 AND revision=$6 AND created_by=$7 AND NOT(paused OR deleted)`, evolutionJSON(compiled), s.Namespace, old.Target.ID, s.ProjectID, old.VersionID, old.Revision, old.OwnerID)
	if err != nil {
		return old, err
	}
	if result.RowsAffected() != 1 {
		return old, evolution.ErrConflict
	}
	current, err := (EvolutionEntries{}).ReadTx(ctx, tx, s, old.Target)
	if err != nil {
		return old, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evolution_remote_intents(effect_key,namespace,project_id,target_id,revision,expected,result,status) VALUES($1,$2,$3,$4,$5,$6,$7,'pending')`, key, s.Namespace, s.ProjectID, old.Target.ID, current.Revision, evolutionJSON(old), evolutionJSON(current))
	return current, err
}
