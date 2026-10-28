package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/identity"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/jackc/pgx/v5"
)

type Fire struct {
	ScheduleID string              `json:"schedule_id"`
	Slot       time.Time           `json:"slot"`
	Status     string              `json:"status"`
	Reason     string              `json:"reason"`
	RequestID  string              `json:"request_id,omitempty"`
	Snapshot   submission.Snapshot `json:"-"`
}

func Claim(ctx context.Context, namespace, id string, revision int, slot time.Time, active func(context.Context, string) (bool, error)) (*Fire, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := transaction(ctx, conn, namespace)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "schedule:"+id); err != nil {
		return nil, err
	}
	plan, err := scanPlan(tx.QueryRow(ctx, "SELECT "+planColumns+" FROM pangu.schedules WHERE id=$1 AND namespace=$2", id, namespace))
	if err != nil {
		return nil, err
	}
	slot = slot.UTC().Truncate(time.Minute)
	existing, err := scanFire(tx.QueryRow(ctx, `SELECT schedule_id::text,slot,status,reason,coalesce(request_id::text,''),snapshot FROM pangu.schedule_fires WHERE schedule_id=$1 AND slot=$2`, id, slot))
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	out := &Fire{ScheduleID: id, Slot: slot, Status: "pending", Snapshot: submission.Snapshot{Workflow: plan.Workflow, CreatedBy: plan.CreatedBy}}
	out.Reason, err = skipReason(ctx, tx, plan, slot, active)
	if revision != plan.Revision && out.Reason == "" {
		out.Reason = "stale_revision"
	}
	if err != nil {
		return nil, err
	}
	if out.Reason != "" {
		out.Status = "skipped"
	}
	data, err := json.Marshal(out.Snapshot)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO pangu.schedule_fires(schedule_id,slot,snapshot,status,reason) VALUES($1,$2,$3,$4,$5)", id, slot, data, out.Status, out.Reason); err != nil {
		return nil, err
	}
	if out.Status == "pending" {
		if _, err = tx.Exec(ctx, "UPDATE pangu.schedules SET active_slot=$2 WHERE id=$1", id, slot); err != nil {
			return nil, err
		}
	}
	return out, tx.Commit(ctx)
}

func skipReason(ctx context.Context, tx pgx.Tx, plan *Plan, slot time.Time, active func(context.Context, string) (bool, error)) (string, error) {
	if plan.Deleted {
		return "deleted", nil
	}
	if plan.Paused {
		return "paused", nil
	}
	if time.Now().UTC().After(slot.Add(time.Minute)) {
		return "missed", nil
	}
	if plan.CreatedBy != "" {
		allowed, err := identity.ScheduleOwner(ctx, tx, plan.CreatedBy, plan.Workflow.ProjectID)
		if err != nil {
			return "", err
		}
		if !allowed {
			return "owner_disabled", nil
		}
	}
	if plan.ActiveSlot == nil {
		return "", nil
	}
	var request string
	err := tx.QueryRow(ctx, "SELECT coalesce(request_id::text,'') FROM pangu.schedule_fires WHERE schedule_id=$1 AND slot=$2", plan.ID, *plan.ActiveSlot).Scan(&request)
	if err != nil {
		return "", err
	}
	if request == "" {
		return "overlap_pending", nil
	}
	running, err := active(ctx, request)
	if err != nil {
		return "status_unknown", nil
	}
	if running {
		return "overlap", nil
	}
	return "", nil
}

func Bind(ctx context.Context, fire *Fire, request string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.schedule_fires SET request_id=$3,status='accepted' WHERE schedule_id=$1 AND slot=$2 AND status='pending'", fire.ScheduleID, fire.Slot, request)
	return err
}

func Skip(ctx context.Context, fire *Fire, reason string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "schedule:"+fire.ScheduleID); err != nil {
		return err
	}
	changed, err := tx.Exec(ctx, "UPDATE pangu.schedule_fires SET status='skipped',reason=$3 WHERE schedule_id=$1 AND slot=$2 AND status='pending' AND request_id IS NULL", fire.ScheduleID, fire.Slot, reason)
	if err != nil {
		return err
	}
	if changed.RowsAffected() == 1 {
		_, err = tx.Exec(ctx, "UPDATE pangu.schedules SET active_slot=NULL WHERE id=$1 AND active_slot=$2", fire.ScheduleID, fire.Slot)
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err == nil && changed.RowsAffected() == 1 {
		fire.Status, fire.Reason = "skipped", reason
	}
	return err
}

func scanFire(row pgx.Row) (*Fire, error) {
	out := &Fire{}
	var data []byte
	err := row.Scan(&out.ScheduleID, &out.Slot, &out.Status, &out.Reason, &out.RequestID, &data)
	if err != nil {
		return nil, err
	}
	return out, json.Unmarshal(data, &out.Snapshot)
}
