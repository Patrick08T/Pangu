package usage

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentReservationsNeverExceedBudget(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ns := uuid.NewString()
	p, err := workspace.Register(ctx, ns, workspace.Project{Name: "预算", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := submission.Prepare(ctx, ns, "submit", "one", submission.Snapshot{Workflow: workflow.Definition{ProjectID: p.ID, Budget: &workflow.Budget{ModelCalls: 2, SearchCalls: 1, CodexSessions: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, ns, item.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	ctx = WithRun(ctx, run, "agent", uuid.NewString(), 1)
	var count atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			id, err := Reserve(ctx, "model", "example", nil)
			if err == nil {
				count.Add(1)
				if id == "" {
					t.Error("缺少预留记录")
				}
			}
		})
	}
	wg.Wait()
	if count.Load() != 2 {
		t.Fatalf("预留=%d", count.Load())
	}
	report, err := ForRun(ctx, run)
	if err != nil || len(report.Calls) != 2 {
		t.Fatalf("调用记录 %v %v", report, err)
	}
	for _, call := range report.Calls {
		if call.InputTokens != nil || call.EstimateMicros != nil || call.Status != "unknown" {
			t.Fatal("未知消耗伪装成零")
		}
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "SELECT pangu.purge_run_payloads($1)", run); err != nil {
		t.Fatal(err)
	}
	report, err = ForRun(ctx, run)
	if err != nil || report.Budget.ModelCalls != 2 || report.Budget.CodexSessions != 1 {
		t.Fatalf("清理使预算失真 %+v %v", report, err)
	}
}

func TestEstimateUsesReportedUsageOnly(t *testing.T) {
	price := &workflow.Price{Currency: "USD", InputPerMillionMicros: 2000000, OutputPerMillionMicros: 6000000}
	if got := Estimate(price, &Tokens{Input: 1000, Output: 500}); got == nil || *got != 5000 {
		t.Fatalf("估算 %v", got)
	}
	if Estimate(price, nil) != nil || Estimate(nil, &Tokens{}) != nil {
		t.Fatal("缺少 usage 或价格仍产生金额")
	}
}
