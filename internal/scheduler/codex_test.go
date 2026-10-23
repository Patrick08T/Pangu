package scheduler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func TestHatchetCodexScheduling(t *testing.T) {
	_, client := integrationClient(t)
	var mu sync.Mutex
	active, maximum := 0, 0
	calls := map[string]int{}
	worker, err := scheduler.NewWorker(client, func(ctx context.Context, in workflow.NodeInput) (workflow.NodeOutput, error) {
		if in.Node.Executor != "codex" {
			return agent.Execute(ctx, in)
		}
		mu.Lock()
		calls[in.Node.ID]++
		attempt := calls[in.Node.ID]
		active++
		maximum = max(maximum, active)
		mu.Unlock()
		defer func() { mu.Lock(); active--; mu.Unlock() }()
		if in.Parents["source"].Text != "source output" {
			return workflow.NodeOutput{}, fmt.Errorf("missing parent")
		}
		time.Sleep(200 * time.Millisecond)
		if in.Node.ID == "codeA" && attempt == 1 {
			return workflow.NodeOutput{}, fmt.Errorf("temporary Codex failure")
		}
		return workflow.NodeOutput{Text: "code output"}, nil
	}, mockPreflight)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })
	backend := scheduler.NewBackend(client)
	awaitWorker(t, backend)
	project := registerTestProject(t, backend)
	h := api.New(backend, integrationAccess{})
	d := workflow.Definition{ProjectID: project.ID, Input: "mixed", Nodes: []workflow.Node{
		{ID: "source", Executor: "eino", Model: "source", Instruction: "source"},
		{ID: "codeA", Executor: "codex", Instruction: "code", DependsOn: []string{"source"}},
		{ID: "codeB", Executor: "codex", Instruction: "code", DependsOn: []string{"source"}},
		{ID: "final", Executor: "eino", Model: "final", Instruction: "final", DependsOn: []string{"codeA", "codeB"}},
	}}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	request(t, h, "POST", "/runs", d, 202, &accepted)
	out := waitRun(t, h, accepted.RunID)
	mu.Lock()
	defer mu.Unlock()
	if maximum != 1 || calls["codeA"] != 2 || calls["codeB"] != 1 {
		t.Fatalf("maximum=%d calls=%v", maximum, calls)
	}
	codexNodes := 0
	for _, node := range out.Nodes {
		if node.Executor == "codex" {
			codexNodes++
			assertCodexQueueTimeout(t, client, node.ID)
		}
	}
	if len(out.Nodes) != 4 || codexNodes != 2 {
		t.Fatalf("nodes=%d codex=%d", len(out.Nodes), codexNodes)
	}
}

func assertCodexQueueTimeout(t *testing.T, client *hatchet.Client, id string) {
	t.Helper()
	var details *rest.V1WorkflowRunDetails
	details, err := client.Runs().Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Tasks []struct{ ScheduleTimeout *string }
	}
	data, err := json.Marshal(details.WorkflowConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &config); err != nil || len(config.Tasks) != 1 {
		t.Fatalf("invalid config: %v", err)
	}
	value := config.Tasks[0].ScheduleTimeout
	if value == nil {
		t.Fatal("Codex 排队超时未设置")
	}
	duration, err := time.ParseDuration(*value)
	if err != nil || duration != 6*time.Hour {
		t.Fatalf("排队超时错误: %v %q", err, *value)
	}
}
