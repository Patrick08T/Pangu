package schedule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRemoteEvolutionRPCSessionHasNoTransactionAndReleasesOnError(t *testing.T) {
	ctx, namespace, plan, conn := remoteEvolutionFixture(t)
	rpcErr := errors.New("fake external RPC failure")
	_, err := ReconcileRemote(ctx, namespace, plan.ID, func(context.Context, *Plan) (string, error) {
		var count int
		var idle bool
		err := conn.QueryRow(ctx, `SELECT count(*),coalesce(bool_and(a.xact_start IS NULL),false)
 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid
 WHERE l.locktype='advisory' AND l.granted AND l.objsubid=1
 AND l.classid::bigint=((hashtextextended($1,0)>>32)&4294967295)
 AND l.objid::bigint=(hashtextextended($1,0)&4294967295)`, "schedule-remote:"+plan.ID).Scan(&count, &idle)
		if err != nil || count != 1 || !idle {
			t.Fatalf("RPC session lock/transaction invalid: locks=%d noTx=%v err=%v", count, idle, err)
		}
		var available bool
		err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "schedule-remote:"+plan.ID).Scan(&available)
		if available {
			remoteEvolutionExec(t, conn, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "schedule-remote:"+plan.ID)
		}
		if err != nil || available {
			t.Fatalf("session mutex not held throughout RPC: available=%v err=%v", available, err)
		}
		return "", rpcErr
	})
	if !errors.Is(err, rpcErr) {
		t.Fatalf("RPC error lost: %v", err)
	}
	if remoteEvolutionCurrent(t, ctx, namespace, plan.ID).Ready {
		t.Fatal("failed RPC wrote receipt")
	}
	var available bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "schedule-remote:"+plan.ID).Scan(&available)
	if available {
		remoteEvolutionExec(t, conn, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "schedule-remote:"+plan.ID)
	}
	if err != nil || !available {
		t.Fatalf("session mutex leaked on RPC error: %v %v", available, err)
	}
}

func TestRemoteEvolutionUUIDAliasCannotBypassSharedMutex(t *testing.T) {
	ctx, namespace, plan, conn := remoteEvolutionFixture(t)
	remoteEvolutionExec(t, conn, `SELECT pg_advisory_lock(hashtextextended($1,0))`, "schedule-remote:"+plan.ID)
	defer remoteEvolutionExec(t, conn, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "schedule-remote:"+plan.ID)
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	_, err := ReconcileRemote(ctx, namespace, strings.ToUpper(plan.ID), func(context.Context, *Plan) (string, error) {
		t.Error("UUID alias bypassed shared mutex")
		return uuid.NewString(), nil
	})
	if err == nil {
		t.Fatal("alias reconciliation unexpectedly succeeded")
	}
}

func TestRemoteEvolutionValidatesNamespaceBeforeRPC(t *testing.T) {
	ctx, namespace, plan, _ := remoteEvolutionFixture(t)
	_, err := ReconcileRemote(ctx, namespace+"-other", plan.ID, func(context.Context, *Plan) (string, error) {
		t.Error("foreign namespace reached RPC")
		return uuid.NewString(), nil
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign namespace accepted: %v", err)
	}
	if remoteEvolutionCurrent(t, ctx, namespace, plan.ID).Ready {
		t.Fatal("foreign namespace wrote receipt")
	}
}

func TestRemoteEvolutionCallbackCannotRewriteExpectedSnapshot(t *testing.T) {
	ctx, namespace, plan, conn := remoteEvolutionFixture(t)
	result, err := ReconcileRemote(ctx, namespace, plan.ID, func(_ context.Context, sent *Plan) (string, error) {
		remoteEvolutionExec(t, conn, `UPDATE pangu.schedules SET revision=revision+1 WHERE id=$1`, plan.ID)
		*sent = *remoteEvolutionCurrent(t, ctx, namespace, plan.ID)
		return uuid.NewString(), nil
	})
	if !errors.Is(err, ErrConflict) || result == nil || result.Ready {
		t.Fatalf("mutable callback rewrote CAS snapshot: %+v %v", result, err)
	}
}
