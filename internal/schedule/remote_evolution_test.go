package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

// Retaining the schedule transaction across RPC prevents this real Save from committing.
func TestRemoteEvolutionRPCAllowsIndependentEdit(t *testing.T) {
	ctx, namespace, before, _ := remoteEvolutionFixture(t)
	var editErr error
	remote := uuid.NewString()
	result, err := ReconcileRemote(ctx, namespace, before.ID, func(ctx context.Context, sent *Plan) (string, error) {
		editCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_, editErr = Save(editCtx, namespace, sent.ID, sent.Revision, workflow.Cron{Name: sent.Name, Expression: "*/5 * * * *", Workflow: sent.Workflow}, false)
		return remote, nil
	})
	if editErr != nil {
		t.Fatalf("RPC could not independently commit human edit: %v", editErr)
	}
	if !errors.Is(err, ErrConflict) || result == nil || result.Ready {
		t.Fatalf("stale receipt accepted: %+v %v", result, err)
	}
	current := remoteEvolutionCurrent(t, ctx, namespace, before.ID)
	if current.Revision != before.Revision+1 || current.Expression != "*/5 * * * *" || current.Ready || current.HatchetID != "" {
		t.Fatalf("human edit overwritten by stale receipt: %+v", current)
	}
	latest, err := ReconcileRemote(ctx, namespace, before.ID, func(_ context.Context, sent *Plan) (string, error) {
		if sent.Revision != current.Revision || sent.Expression != current.Expression {
			t.Errorf("retry did not read latest plan: %+v", sent)
		}
		return remote, nil
	})
	if err != nil || !latest.Ready || latest.HatchetID != remote {
		t.Fatalf("current retry failed: %+v %v", latest, err)
	}
}

func TestRemoteEvolutionRPCPreservesHumanDeletion(t *testing.T) {
	ctx, namespace, before, _ := remoteEvolutionFixture(t)
	var deleteErr error
	result, err := ReconcileRemote(ctx, namespace, before.ID, func(ctx context.Context, _ *Plan) (string, error) {
		editCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		deleteErr = Delete(editCtx, namespace, before.ID)
		return uuid.NewString(), nil
	})
	if deleteErr != nil {
		t.Fatalf("RPC blocked human deletion: %v", deleteErr)
	}
	if !errors.Is(err, ErrConflict) || result == nil || result.Ready {
		t.Fatalf("deleted snapshot confirmed: %+v %v", result, err)
	}
	current := remoteEvolutionCurrent(t, ctx, namespace, before.ID)
	if !current.Deleted || !current.Paused || current.Ready || current.HatchetID != "" {
		t.Fatalf("deletion lost: %+v", current)
	}
	result, err = ReconcileRemote(ctx, namespace, before.ID, func(_ context.Context, sent *Plan) (string, error) {
		if !sent.Deleted || !sent.Paused {
			t.Error("retry did not send remote deletion")
		}
		return "", nil
	})
	if err != nil || !result.Ready || !result.Deleted || result.HatchetID != "" {
		t.Fatalf("delete reconciliation failed: %+v %v", result, err)
	}
}

func TestRemoteEvolutionRPCPreservesPauseWithoutRevisionChange(t *testing.T) {
	ctx, namespace, before, conn := remoteEvolutionFixture(t)
	result, err := ReconcileRemote(ctx, namespace, before.ID, func(_ context.Context, _ *Plan) (string, error) {
		remoteEvolutionExec(t, conn, `UPDATE pangu.schedules SET paused=true,remote_synced=false WHERE id=$1`, before.ID)
		return uuid.NewString(), nil
	})
	if !errors.Is(err, ErrConflict) || result == nil || !result.Paused || result.Ready {
		t.Fatalf("pause overwritten: %+v %v", result, err)
	}
	current := remoteEvolutionCurrent(t, ctx, namespace, before.ID)
	if !current.Paused || current.Ready || current.Revision != before.Revision {
		t.Fatalf("pause changed: %+v", current)
	}
}

func TestRemoteEvolutionRPCPreservesLaterOwnerReceipt(t *testing.T) {
	ctx, namespace, before, conn := remoteEvolutionFixture(t)
	ownerRemote := uuid.NewString()
	result, err := ReconcileRemote(ctx, namespace, before.ID, func(_ context.Context, _ *Plan) (string, error) {
		remoteEvolutionExec(t, conn, `UPDATE pangu.schedules SET remote_synced=true,hatchet_id=$2 WHERE id=$1`, before.ID, ownerRemote)
		return uuid.NewString(), nil
	})
	if !errors.Is(err, ErrConflict) || result == nil || !result.Ready || result.HatchetID != ownerRemote {
		t.Fatalf("later receipt replaced: %+v %v", result, err)
	}
	current := remoteEvolutionCurrent(t, ctx, namespace, before.ID)
	if !current.Ready || current.HatchetID != ownerRemote {
		t.Fatalf("later owner receipt lost: %+v", current)
	}
}

func TestRemoteEvolutionUsesSharedSessionMutex(t *testing.T) {
	ctx, namespace, before, conn := remoteEvolutionFixture(t)
	remoteEvolutionExec(t, conn, `SELECT pg_advisory_lock(hashtextextended($1,0))`, "schedule-remote:"+before.ID)
	defer remoteEvolutionExec(t, conn, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "schedule-remote:"+before.ID)
	blockedCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	called := false
	_, err := ReconcileRemote(blockedCtx, namespace, before.ID, func(context.Context, *Plan) (string, error) {
		called = true
		return uuid.NewString(), nil
	})
	if called || err == nil {
		t.Fatalf("ordinary reconciliation bypassed evolution session lock: called=%v err=%v", called, err)
	}
	if remoteEvolutionCurrent(t, ctx, namespace, before.ID).Ready {
		t.Fatal("blocked reconciliation wrote receipt")
	}
}
