package scheduler_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

// Only diagnostic tasks in a fresh namespace; no product database or model calls.
func preflightBudgetWorker(t *testing.T, inspect func(context.Context, workflow.Definition) workflow.Preflight) scheduler.Backend {
	t.Helper()
	if os.Getenv("PANGU_PREFLIGHT_INTEGRATION") != "1" {
		t.Skip("requires explicit PANGU_PREFLIGHT_INTEGRATION=1 and local Hatchet")
	}
	t.Setenv("PANGU_DATABASE_URL", "")
	t.Setenv("HATCHET_CLIENT_NAMESPACE", fmt.Sprintf("preflight_budget_test_%d", time.Now().UnixNano()))
	client, err := hatchet.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	var executions atomic.Int32
	worker, err := scheduler.NewWorker(client, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		executions.Add(1)
		return workflow.NodeOutput{}, fmt.Errorf("preflight must not execute business nodes")
	}, inspect)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stop()
		if executions.Load() != 0 {
			t.Errorf("preflight executed %d business nodes", executions.Load())
		}
	})
	backend := scheduler.NewBackend(client)
	awaitWorker(t, backend)
	return backend
}
