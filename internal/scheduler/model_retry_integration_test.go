package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetModelRetrySurvivesRestartWithZeroNodeRetries(t *testing.T) {
	_, client := integrationClient(t)
	b := scheduler.NewBackend(client)
	p := registerTestProject(t, b)
	var mu sync.Mutex
	counts := map[string]int{}
	execute := func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		mu.Lock()
		defer mu.Unlock()
		counts[input.Node.ID]++
		if input.Node.ID == "model" && counts["model"] == 1 {
			return workflow.NodeOutput{}, &workflow.Failure{Kind: "model_unavailable", Retry: true, Cause: errors.New("模型容量不足")}
		}
		return workflow.NodeOutput{Text: "保留结果"}, nil
	}
	stop := startReviewWorker(t, client, execute)
	t.Cleanup(func() {
		if stop != nil {
			_ = stop()
		}
	})
	d := retryDefinition(p.ID)
	r, err := b.Submit(context.Background(), d, "model-retry-restart")
	if err != nil {
		t.Fatal(err)
	}
	awaitModelFailure(t, b, r.RunID)
	if err = stop(); err != nil {
		t.Fatal(err)
	}
	stop = nil
	stop = startReviewWorker(t, client, execute)
	out := waitRun(t, api.New(b, integrationAccess{}), r.RunID)
	mu.Lock()
	defer mu.Unlock()
	if counts["source"] != 1 || counts["model"] != 2 || counts["sibling"] != 1 || counts["end"] != 1 {
		t.Fatalf("完成节点被重复执行: %v", counts)
	}
	var failed, retried time.Time
	for _, a := range out.Attempts {
		if a.NodeID == "model" && a.Status == "failed" {
			failed = *a.FinishedAt
		}
		if a.NodeID == "model" && a.Status == "succeeded" {
			retried = a.StartedAt
		}
	}
	if retried.Sub(failed) < 29*time.Second {
		t.Fatal("模型故障未按 30 秒退避")
	}
}

func retryDefinition(project string) workflow.Definition {
	d := workflow.Definition{ProjectID: project}
	for _, id := range []string{"source", "model", "sibling", "end"} {
		n := workflow.Node{ID: id, Executor: "eino", Model: "test", Instruction: "验证模型重试", Handoff: testHandoff(), Policy: &workflow.Policy{Retries: 0}}
		if id == "model" || id == "sibling" {
			n.DependsOn = []string{"source"}
		}
		if id == "end" {
			n.DependsOn = []string{"model", "sibling"}
		}
		d.Nodes = append(d.Nodes, n)
	}
	return d
}

func awaitModelFailure(t *testing.T, b scheduler.Backend, run string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		items, err := execution.Attempts(context.Background(), run)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range items {
			if a.Category == "model_unavailable" && a.FinishedAt != nil {
				time.Sleep(time.Second)
				state, err := b.GetRun(context.Background(), run)
				if err != nil || state.StopConfirmed || state.Status == "failed" {
					t.Fatalf("模型暂时失败终止工作流: %+v %v", state, err)
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("未观测到模型故障")
}
