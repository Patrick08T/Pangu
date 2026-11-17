package scheduler_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetPreflightAcceptsSlowHealthyInspection(t *testing.T) {
	var calls atomic.Int32
	backend := preflightBudgetWorker(t, func(ctx context.Context, d workflow.Definition) workflow.Preflight {
		if d.Input == "slow" {
			calls.Add(1)
			select {
			case <-time.After(11 * time.Second):
			case <-ctx.Done():
				return workflow.Preflight{}
			}
		}
		return workflow.Preflight{Ready: true}
	})
	result, err := backend.Preflight(t.Context(), workflow.Definition{Input: "slow"})
	if err != nil || result == nil || !result.Ready || calls.Load() != 1 {
		t.Fatalf("healthy >8s inspection rejected or retried: result=%+v err=%v calls=%d", result, err, calls.Load())
	}
}

func TestHatchetPreflightStillStopsStalledInspection(t *testing.T) {
	var calls atomic.Int32
	stopped := make(chan struct{})
	backend := preflightBudgetWorker(t, func(ctx context.Context, d workflow.Definition) workflow.Preflight {
		if d.Input == "stalled" {
			calls.Add(1)
			<-ctx.Done()
			close(stopped)
			return workflow.Preflight{}
		}
		return workflow.Preflight{Ready: true}
	})
	started := time.Now()
	result, err := backend.Preflight(t.Context(), workflow.Definition{Input: "stalled"})
	if !errors.Is(err, scheduler.ErrNotReady) || result != nil || time.Since(started) >= 30*time.Second {
		t.Fatalf("stalled inspection did not fail within HTTP budget: result=%+v err=%v elapsed=%s", result, err, time.Since(started))
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not cancel stalled inspector")
	}
	if calls.Load() != 1 {
		t.Fatalf("stalled inspection retried: %d", calls.Load())
	}
}
