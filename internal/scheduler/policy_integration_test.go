package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetNodePoliciesControlRetriesAndDeadlines(t *testing.T) {
	_, client := integrationClient(t)
	var mu sync.Mutex
	counts := map[string]int{}
	worker, err := scheduler.NewWorker(client, func(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		mu.Lock()
		counts[input.Node.ID]++
		mu.Unlock()
		switch input.Node.ID {
		case "once", "retry":
			return workflow.NodeOutput{}, errors.New("临时故障")
		case "auth":
			return workflow.NodeOutput{}, &workflow.Failure{Kind: "authentication", Cause: errors.New("凭据失效")}
		default:
			select {
			case <-ctx.Done():
				return workflow.NodeOutput{}, ctx.Err()
			case <-time.After(2 * time.Second):
				return workflow.NodeOutput{Text: "完成"}, nil
			}
		}
	}, mockPreflight)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	b := scheduler.NewBackend(client)
	awaitWorker(t, b)
	p := registerTestProject(t, b)
	d := workflow.Definition{ProjectID: p.ID}
	for _, item := range []struct {
		id               string
		retries, seconds int
	}{{"once", 0, 10}, {"retry", 2, 10}, {"short", 0, 1}, {"long", 0, 5}, {"auth", 2, 10}} {
		d.Nodes = append(d.Nodes, workflow.Node{ID: item.id, Executor: "eino", Model: "test", Instruction: "策略验证", Policy: &workflow.Policy{Retries: item.retries, TimeoutSeconds: item.seconds, ToolCalls: 0}})
	}
	record, err := b.Submit(context.Background(), d, "node-policy")
	if err != nil {
		t.Fatal(err)
	}
	run := waitFailedRun(t, b, record.RunID)
	mu.Lock()
	defer mu.Unlock()
	if counts["once"] != 1 || counts["retry"] != 3 || counts["auth"] != 1 || counts["short"] != 1 || counts["long"] != 1 {
		t.Fatalf("策略未实际生效: %+v", counts)
	}
	for _, node := range run.Nodes {
		if node.NodeID == "long" && node.Status != "succeeded" {
			t.Fatal("同层长超时节点未收敛")
		}
	}
	for _, attempt := range run.Attempts {
		if attempt.NodeID == "short" && attempt.Category != "timeout" {
			t.Fatalf("超时分类丢失: %+v", attempt)
		}
	}
}

func waitFailedRun(t *testing.T, b scheduler.Backend, id string) *scheduler.RunDetails {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		run, err := b.GetRun(context.Background(), id)
		if err == nil && run.Status == "failed" && run.StopConfirmed {
			return run
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("未在期限内收敛失败运行")
	return nil
}
