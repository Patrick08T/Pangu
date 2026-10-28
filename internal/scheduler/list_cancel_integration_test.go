package scheduler_test

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
	"time"
)

func TestHatchetListUsesConfirmedCancellationAndProjectFilter(t *testing.T) {
	_, client := integrationClient(t)
	started, release := make(chan struct{}), make(chan struct{})
	worker, err := scheduler.NewWorker(client, func(ctx context.Context, _ workflow.NodeInput) (workflow.NodeOutput, error) {
		close(started)
		<-ctx.Done()
		<-release
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
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	b := scheduler.NewBackend(client)
	awaitWorker(t, b)
	p := registerTestProject(t, b)
	accepted, err := b.Submit(context.Background(), workflow.Definition{ProjectID: p.ID, Nodes: []workflow.Node{{ID: "hold", Executor: "codex", Handoff: testHandoff(), Instruction: "取消后等待停止证据"}}}, "list-cancel")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("执行器未启动")
	}
	if _, err = b.CancelRun(context.Background(), accepted.RunID); err != nil {
		t.Fatal(err)
	}
	q := scheduler.RunQuery{ProjectID: p.ID, Status: "cancelling", Since: time.Now().Add(-time.Hour), Until: time.Now(), Limit: 20}
	page, err := b.ListRuns(context.Background(), q)
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != "cancelling" {
		t.Fatalf("未证停却显示安全取消: %+v %v", page, err)
	}
	q.ProjectID = "00000000-0000-0000-0000-000000000001"
	page, err = b.ListRuns(context.Background(), q)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("项目筛选失效: %+v %v", page, err)
	}
	close(release)
	released = true
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err = b.Reconcile(context.Background()); err != nil {
			t.Fatal(err)
		}
		d, err := b.GetRun(context.Background(), accepted.RunID)
		if err == nil && d.StopConfirmed && d.Status == "cancelled" {
			q.ProjectID = p.ID
			q.Status = "cancelled"
			page, err = b.ListRuns(context.Background(), q)
			if err != nil || len(page.Items) != 1 {
				t.Fatalf("终态筛选与详情不一致: %+v %v", page, err)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("取消后未收敛")
}
