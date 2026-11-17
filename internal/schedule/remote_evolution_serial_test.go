package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRemoteEvolutionConcurrentReconcilesSerializeRPC(t *testing.T) {
	ctx, namespace, plan, _ := remoteEvolutionFixture(t)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	type outcome struct {
		plan *Plan
		err  error
	}
	done := make(chan outcome, 1)
	remote := uuid.NewString()
	go func() {
		result, err := ReconcileRemote(ctx, namespace, plan.ID, func(ctx context.Context, _ *Plan) (string, error) {
			close(entered)
			select {
			case <-release:
				return remote, nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		})
		done <- outcome{result, err}
	}()
	select {
	case <-entered:
	case first := <-done:
		t.Fatalf("first reconcile failed before RPC: %v", first.err)
	case <-ctx.Done():
		t.Fatal("first RPC did not start")
	}
	blocked, unblock := context.WithTimeout(ctx, 300*time.Millisecond)
	_, err := ReconcileRemote(blocked, namespace, plan.ID, func(context.Context, *Plan) (string, error) {
		t.Error("second RPC overlapped first RPC")
		return uuid.NewString(), nil
	})
	unblock()
	close(release)
	first := <-done
	if err == nil {
		t.Error("second reconcile bypassed mutex")
	}
	if first.err != nil || first.plan == nil || !first.plan.Ready || first.plan.HatchetID != remote {
		t.Fatalf("first receipt failed: %+v %v", first.plan, first.err)
	}
	result, err := ReconcileRemote(ctx, namespace, plan.ID, func(context.Context, *Plan) (string, error) {
		t.Error("already confirmed snapshot invoked RPC again")
		return "", nil
	})
	if err != nil || result == nil || !result.Ready || result.HatchetID != remote {
		t.Fatalf("serialized current read failed: %+v %v", result, err)
	}
}
