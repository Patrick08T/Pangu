package scheduler_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/usage"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetModelRetryCancellationStopsBeforeNextCall(t *testing.T) {
	_, client := integrationClient(t)
	b := scheduler.NewBackend(client)
	p := registerTestProject(t, b)
	var calls atomic.Int32
	stop := startReviewWorker(t, client, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		calls.Add(1)
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "model_unavailable", Retry: true, Cause: errors.New("容量不足")}
	})
	defer stop()
	d := retryDefinition(p.ID)
	d.Nodes = d.Nodes[:1]
	r, err := b.Submit(context.Background(), d, "cancel-model-retry")
	if err != nil {
		t.Fatal(err)
	}
	awaitModelFailure(t, b, r.RunID)
	state, err := b.GetRun(context.Background(), r.RunID)
	if err != nil || len(state.Nodes) != 1 || state.Nodes[0].NextRetryAt == nil {
		t.Fatalf("没有公开等待状态: %+v %v", state, err)
	}
	if _, err = b.CancelRun(context.Background(), r.RunID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		state, err = b.GetRun(context.Background(), r.RunID)
		if err == nil && state.StopConfirmed {
			if calls.Load() != 1 || state.Nodes[0].NextRetryAt != nil {
				t.Fatal("取消后仍有模型重试")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("等待重试的运行无法及时取消")
}

func TestHatchetModelRetryHonorsExplicitBudget(t *testing.T) {
	_, client := integrationClient(t)
	b := scheduler.NewBackend(client)
	p := registerTestProject(t, b)
	var calls atomic.Int32
	stop := startReviewWorker(t, client, func(ctx context.Context, _ workflow.NodeInput) (workflow.NodeOutput, error) {
		id, err := usage.Reserve(ctx, "model", "test", nil)
		if err != nil {
			return workflow.NodeOutput{}, err
		}
		calls.Add(1)
		if err = usage.Finish(id, nil, false); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "model_unavailable", Retry: true, Cause: errors.New("容量不足")}
	})
	defer stop()
	d := retryDefinition(p.ID)
	d.Nodes = d.Nodes[:1]
	d.Budget = &workflow.Budget{ModelCalls: 1}
	r, err := b.Submit(context.Background(), d, "budget-model-retry")
	if err != nil {
		t.Fatal(err)
	}
	final := waitFailedRun(t, b, r.RunID)
	if calls.Load() != 1 || len(final.Attempts) != 2 || final.Attempts[1].Category != "budget" {
		t.Fatalf("预算没有终止模型重试: %+v", final.Attempts)
	}
	for _, node := range final.Nodes {
		if node.NextRetryAt != nil {
			t.Fatal("终态仍显示等待重试")
		}
	}
}
