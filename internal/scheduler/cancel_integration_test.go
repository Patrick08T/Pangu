package scheduler_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetCancellationPropagation(t *testing.T) {
	_, client := integrationClient(t)
	started, stopped := make(chan struct{}, 2), make(chan struct{}, 2)
	var calls atomic.Int32
	worker, err := scheduler.NewWorker(client, func(ctx context.Context, _ workflow.NodeInput) (workflow.NodeOutput, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
		return workflow.NodeOutput{}, ctx.Err()
	}, mockPreflight)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	backend := scheduler.NewBackend(client)
	awaitWorker(t, backend)
	project := registerTestProject(t, backend)
	accepted, err := backend.Submit(context.Background(), workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{
		{ID: "a", Executor: "codex", Instruction: "阻塞"}, {ID: "b", Executor: "codex", Instruction: "排队"},
	}}, "cancel-test")
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.RunID
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("子节点未启动")
	}
	if _, err := backend.CancelRun(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(20 * time.Second):
		t.Fatal("取消根运行没有将取消传播给正在执行的子任务")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := backend.Reconcile(context.Background()); err != nil {
			t.Fatal(err)
		}
		state, err := backend.GetRun(context.Background(), id)
		if err == nil && state.Status == "cancelled" && state.StopConfirmed && len(state.Nodes) == 2 && state.Nodes[0].Status == "cancelled" && state.Nodes[1].Status == "cancelled" {
			if calls.Load() != 1 {
				t.Fatalf("排队子节点不应执行，次数=%d", calls.Load())
			}
			t.Logf("取消传播通过，根与两个子任务均取消，执行器收到 context 取消: %s", id)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("根与排队/执行子任务未全部进入 cancelled")
}

func mockPreflight(context.Context, workflow.Definition) workflow.Preflight {
	return workflow.Preflight{Ready: true, Checks: []workflow.Check{}}
}
