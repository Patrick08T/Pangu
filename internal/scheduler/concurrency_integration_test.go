package scheduler_test

import (
	"context"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
	"time"
)

func TestHatchetIndependentCodexRunsExecuteConcurrently(t *testing.T) {
	_, client := integrationClient(t)
	started := make(chan string, 2)
	release := make(chan struct{})
	worker, err := scheduler.NewWorker(client, func(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		started <- input.RunID
		select {
		case <-release:
			return workflow.NodeOutput{Text: "完成"}, nil
		case <-ctx.Done():
			return workflow.NodeOutput{}, ctx.Err()
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
	defer close(release)
	b := scheduler.NewBackend(client)
	awaitWorker(t, b)
	project := registerTestProject(t, b)
	ids := []string{}
	for n := range 2 {
		record, err := b.Submit(context.Background(), workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "code", Executor: "codex", Handoff: testHandoff(), Instruction: "并行"}}}, fmt.Sprint("parallel-", n))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, record.RunID)
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(15 * time.Second):
			t.Fatal("跨运行仍受 Codex 串行上限限制")
		}
	}
	release <- struct{}{}
	release <- struct{}{}
	for _, id := range ids {
		result := waitRun(t, api.New(b, integrationAccess{}), id)
		if result.Status != "succeeded" {
			t.Fatalf("独立运行失败: %+v", result)
		}
	}
}
