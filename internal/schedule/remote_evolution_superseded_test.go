package schedule

import (
	"context"
	"slices"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestRemoteEvolutionHumanChangesSupersedePendingOwner(t *testing.T) {
	for _, change := range []string{"edit", "delete"} {
		t.Run(change, func(t *testing.T) {
			ctx, namespace, before, conn := remoteEvolutionFixture(t)
			key := remoteEvolutionIntent(t, conn, namespace, before, "pending")
			if change == "edit" {
				_, err := Save(ctx, namespace, before.ID, before.Revision, workflow.Cron{Name: before.Name, Expression: "*/5 * * * *", Workflow: before.Workflow}, false)
				if err != nil {
					t.Fatal(err)
				}
			} else if err := Delete(ctx, namespace, before.ID); err != nil {
				t.Fatal(err)
			}
			ids, err := RemotePending(ctx, namespace)
			if err != nil || !slices.Contains(ids, before.ID) {
				t.Fatalf("obsolete pending owner blocked human change: %v %v", ids, err)
			}
			result, err := ReconcileRemote(ctx, namespace, before.ID, func(_ context.Context, sent *Plan) (string, error) {
				if change == "delete" {
					if !sent.Deleted || !sent.Paused {
						t.Error("human deletion not sent")
					}
					return "", nil
				}
				if sent.Revision != before.Revision+1 || sent.Expression != "*/5 * * * *" {
					t.Error("human edit not sent")
				}
				return uuid.NewString(), nil
			})
			if err != nil || result == nil || !result.Ready {
				t.Fatalf("human change failed to reconcile: %+v %v", result, err)
			}
			var status string
			if err := conn.QueryRow(ctx, `SELECT status FROM pangu.evolution_remote_intents WHERE effect_key=$1`, key).Scan(&status); err != nil || status != "pending" {
				t.Fatalf("ordinary path modified obsolete owner's outbox: %s %v", status, err)
			}
		})
	}
}

func TestRemoteEvolutionOperationalBookkeepingDoesNotInvalidateReceipt(t *testing.T) {
	ctx, namespace, plan, conn := remoteEvolutionFixture(t)
	result, err := ReconcileRemote(ctx, namespace, plan.ID, func(context.Context, *Plan) (string, error) {
		remoteEvolutionExec(t, conn, `UPDATE pangu.schedules SET active_slot=now(),remote_checked_at=now() WHERE id=$1`, plan.ID)
		return uuid.NewString(), nil
	})
	if err != nil || result == nil || !result.Ready || result.ActiveSlot == nil {
		t.Fatalf("operational polling invalidated valid receipt: %+v %v", result, err)
	}
}
