package scheduler

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/jackc/pgx/v5"
)

func evolutionSameEntry(a, b evolution.EntrySnapshot) bool {
	a.RemoteState = ""
	b.RemoteState = ""
	return a == b
}

func evolutionRemoteReceiptTx(ctx context.Context, tx pgx.Tx, namespace, key string, expected evolution.EntrySnapshot, remote string) error {
	scope := evolution.Scope{Namespace: namespace, ProjectID: expected.ProjectID, ActorID: expected.OwnerID}
	current, err := (EvolutionEntries{}).ReadTx(ctx, tx, scope, expected.Target)
	if err != nil {
		return err
	}
	if !evolutionSameEntry(current, expected) || !current.Enabled {
		return evolution.ErrConflict
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.evolution_remote_intents SET status='confirmed',remote_id=$4,finished_at=clock_timestamp() WHERE namespace=$1 AND effect_key=$2 AND result=$3 AND status IN ('running','pending','unknown','confirmed')`, namespace, key, evolutionJSON(expected), remote)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return evolution.ErrConflict
	}
	result, err = tx.Exec(ctx, `UPDATE pangu.schedules SET remote_synced=true,hatchet_id=$1 WHERE namespace=$2 AND id=$3 AND revision=$4 AND workflow->>'version_id'=$5 AND NOT(paused OR deleted) AND created_by=$6`, remote, namespace, current.Target.ID, current.Revision, current.VersionID, current.OwnerID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return evolution.ErrConflict
	}
	return nil
}

// The session mutex serializes remote attempts, but no SQL transaction crosses RPC.
func (b Backend) evolutionReconcileRemote(ctx context.Context, key string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var target string
	if err = conn.QueryRow(ctx, `SELECT target_id::text FROM pangu.evolution_remote_intents WHERE namespace=$1 AND effect_key=$2`, b.Namespace(), key).Scan(&target); err != nil {
		return err
	}
	// Share this target-level mutex with ordinary cron reconciliation and all generations.
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, "schedule-remote:"+target); err != nil {
		return err
	}
	var plan *schedule.Plan
	var expected evolution.EntrySnapshot
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		var e error
		plan, expected, e = evolutionPrepareRemoteTx(ctx, tx, b.Namespace(), key)
		return e
	})
	if err != nil {
		return err
	}
	if plan == nil {
		return nil
	}
	remote, err := b.syncCron(ctx, plan)
	if err != nil {
		_, mark := conn.Exec(context.WithoutCancel(ctx), `UPDATE pangu.evolution_remote_intents SET status='unknown' WHERE namespace=$1 AND effect_key=$2 AND result=$3 AND status='running'`, b.Namespace(), key, evolutionJSON(expected))
		return errors.Join(err, mark)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := evolutionRemoteAuthorityTx(ctx, tx, b.Namespace(), key, expected); err != nil {
			return err
		}
		return evolutionRemoteReceiptTx(ctx, tx, b.Namespace(), key, expected, remote)
	})
	if errors.Is(err, evolution.ErrConflict) {
		_, mark := conn.Exec(context.WithoutCancel(ctx), `UPDATE pangu.evolution_remote_intents SET status='conflict',finished_at=clock_timestamp() WHERE namespace=$1 AND effect_key=$2 AND result=$3 AND status<>'confirmed'`, b.Namespace(), key, evolutionJSON(expected))
		return errors.Join(err, mark)
	}
	return err
}

func evolutionPrepareRemoteTx(ctx context.Context, tx pgx.Tx, namespace, key string) (*schedule.Plan, evolution.EntrySnapshot, error) {
	var expected evolution.EntrySnapshot
	var raw []byte
	var status string
	err := tx.QueryRow(ctx, `SELECT result,status FROM pangu.evolution_remote_intents WHERE namespace=$1 AND effect_key=$2`, namespace, key).Scan(&raw, &status)
	if err != nil {
		return nil, expected, err
	}
	if err = json.Unmarshal(raw, &expected); err != nil {
		return nil, expected, err
	}
	if status == "confirmed" || status == "conflict" {
		return nil, expected, nil
	}
	if err = evolutionRemoteAuthorityTx(ctx, tx, namespace, key, expected); err != nil {
		return nil, expected, err
	}
	scope := evolution.Scope{Namespace: namespace, ProjectID: expected.ProjectID, ActorID: expected.OwnerID}
	current, err := (EvolutionEntries{}).ReadTx(ctx, tx, scope, expected.Target)
	if err != nil {
		return nil, expected, err
	}
	if !evolutionSameEntry(current, expected) || !current.Enabled {
		return nil, expected, evolution.ErrConflict
	}
	if _, err = catalog.ReadVersionScopedTx(ctx, tx, namespace, current.VersionID, catalog.VersionScope{ProjectID: current.ProjectID, AllowObserving: true}); err != nil {
		return nil, expected, err
	}
	plan := &schedule.Plan{ID: current.Target.ID, CreatedBy: current.OwnerID, Revision: int(current.Revision)}
	err = tx.QueryRow(ctx, `SELECT name,expression,workflow,coalesce(hatchet_id::text,'') FROM pangu.schedules WHERE namespace=$1 AND id=$2`, namespace, current.Target.ID).Scan(&plan.Name, &plan.Expression, &raw, &plan.HatchetID)
	if err != nil {
		return nil, expected, err
	}
	if err = json.Unmarshal(raw, &plan.Workflow); err != nil {
		return nil, expected, err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.evolution_remote_intents SET status='running' WHERE namespace=$1 AND effect_key=$2`, namespace, key)
	return plan, expected, err
}

func evolutionRemoteAuthorityTx(ctx context.Context, tx pgx.Tx, namespace, key string, expected evolution.EntrySnapshot) error {
	if err := identity.Lock(ctx, tx, namespace); err != nil {
		return err
	}
	if err := experience.Lock(ctx, tx, namespace, expected.ProjectID); err != nil {
		return err
	}
	var allowed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_intents i JOIN pangu.evolution_switches sw ON sw.intent_id=i.id AND sw.generation=i.generation
 JOIN LATERAL(SELECT * FROM pangu.experience_policies WHERE namespace=i.namespace AND project_id=i.project_id ORDER BY revision DESC LIMIT 1) p ON true
 WHERE i.namespace=$1 AND i.project_id=$2 AND i.id::text||':'||i.generation::text||':'||sw.entry_key||':'||sw.direction=$3 AND sw.result=$4 AND sw.status IN ('pending','unknown')
 AND ((sw.direction='adopt' AND i.state='switching'
 AND EXISTS(SELECT 1 FROM pangu.evaluation_experiments x JOIN LATERAL(SELECT * FROM pangu.evolution_environments WHERE namespace=i.namespace AND project_id=i.project_id AND id::text=x.spec->>'environment_id' ORDER BY revision DESC LIMIT 1) env ON env.spec_hash=x.spec->>'environment_hash'
 JOIN pangu.catalog_versions cv ON cv.id::text=env.spec->>'connection_version' JOIN pangu.catalog_objects co ON co.id=cv.object_id AND co.namespace=i.namespace AND NOT co.archived
 WHERE x.id=i.experiment_id AND x.namespace=i.namespace AND x.project_id=i.project_id AND x.generation=i.generation AND x.status='active' AND x.spec_hash=i.experiment_hash AND env.spec->>'enabled'='true')
 AND p.revision=i.policy_revision AND p.policy->>'enabled'='true' AND p.policy->>'revoked'<>'true' AND EXISTS(SELECT 1 FROM pangu.users u WHERE u.id::text=p.actor_id AND u.namespace=i.namespace AND NOT u.disabled AND (u.is_admin OR EXISTS(SELECT 1 FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=i.project_id AND m.enabled AND m.role='manager')))) OR (sw.direction='rollback' AND i.state IN ('rolling_back','reconciling'))))`, namespace, expected.ProjectID, key, evolutionJSON(expected)).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return evolution.ErrConflict
	}
	if err = evolutionPublishedForwardTx(ctx, tx, namespace, key, expected); err != nil {
		return err
	}
	_, err = catalog.ReadVersionScopedTx(ctx, tx, namespace, expected.VersionID, catalog.VersionScope{ProjectID: expected.ProjectID, AllowObserving: true})
	return err
}
