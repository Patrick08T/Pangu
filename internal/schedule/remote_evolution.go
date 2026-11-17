package schedule

import (
	"bytes"
	"context"

	"github.com/jackc/pgx/v5"
)

// result is the post-CAS snapshot; expected describes the superseded revision.
// Ignore terminal/obsolete intents, but leave a matching live intent to its owner.
// EntryTarget preserves UUID spelling; accept the same UUID forms as uuid.Parse
// without casting untrusted JSON to UUID (malformed old snapshots must not fail).
const remoteEvolutionOwned = `EXISTS (
 SELECT 1 FROM pangu.evolution_remote_intents r
 WHERE r.namespace=s.namespace AND r.project_id=s.project_id AND r.target_id=s.id
 AND r.revision=s.revision AND r.status IN ('pending','running','unknown')
 AND lower(r.result#>>'{target,id}')=ANY(ARRAY[s.id::text,replace(s.id::text,'-',''),
  '{'||s.id::text||'}','urn:uuid:'||s.id::text])
 AND r.result @> jsonb_build_object(
  'target',jsonb_build_object('kind','cron'),
  'project_id',s.project_id::text,'version_id',s.workflow->>'version_id',
  'revision',s.revision,'owner_id',coalesce(s.created_by::text,''),
  'enabled',NOT(s.paused OR s.deleted)))`

// Operational polling/slot bookkeeping is not part of the remote configuration.
// Keep receipt fields in the snapshot so a later owner's receipt cannot be lost.
const remoteEvolutionSnapshot = `(to_jsonb(s)-'remote_checked_at'-'active_slot')`

func remoteEvolutionReadTx(ctx context.Context, tx pgx.Tx, namespace, id string) (*Plan, []byte, bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "schedule:"+id); err != nil {
		return nil, nil, false, err
	}
	plan, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planColumns+` FROM pangu.schedules WHERE namespace=$1 AND id=$2 FOR UPDATE`, namespace, id))
	if err != nil {
		return plan, nil, false, err
	}
	var snapshot []byte
	var owned bool
	err = tx.QueryRow(ctx, `SELECT `+remoteEvolutionSnapshot+`,`+remoteEvolutionOwned+` FROM pangu.schedules s WHERE namespace=$1 AND id=$2 AND project_id::text=workflow->>'project_id'`, namespace, id).Scan(&snapshot, &owned)
	return plan, snapshot, owned, err
}

func remoteEvolutionPrepare(ctx context.Context, conn *pgx.Conn, namespace, id string) (*Plan, []byte, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(context.Background())
	plan, snapshot, owned, err := remoteEvolutionReadTx(ctx, tx, namespace, id)
	if err != nil {
		return plan, nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE pangu.schedules SET remote_checked_at=now() WHERE namespace=$1 AND id=$2`, namespace, id); err != nil {
		return plan, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return plan, nil, err
	}
	if owned {
		return plan, nil, ErrConflict
	}
	return plan, snapshot, nil
}

func remoteEvolutionReceipt(ctx context.Context, conn *pgx.Conn, namespace, id string, expected []byte, remote string) (*Plan, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	current, snapshot, owned, err := remoteEvolutionReadTx(ctx, tx, namespace, id)
	if err != nil {
		return current, err
	}
	if owned || !bytes.Equal(snapshot, expected) {
		return current, ErrConflict
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.schedules s SET remote_synced=true,hatchet_id=nullif($6,'')::uuid
 WHERE namespace=$1 AND id=$2 AND project_id=$3 AND revision=$4 AND `+remoteEvolutionSnapshot+`=$5::jsonb
 AND NOT `+remoteEvolutionOwned, namespace, id, current.Workflow.ProjectID, current.Revision, expected, remote)
	if err != nil {
		return current, err
	}
	if result.RowsAffected() != 1 {
		return current, ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return current, err
	}
	current.Ready, current.HatchetID = true, remote
	return current, nil
}
