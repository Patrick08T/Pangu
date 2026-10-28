package usage

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func httpScope(t *testing.T) (context.Context, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ns := uuid.NewString()
	p, err := workspace.Register(ctx, ns, workspace.Project{Name: "HTTP预算", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := submission.Prepare(ctx, ns, "submit", "http", submission.Snapshot{Workflow: workflow.Definition{ProjectID: p.ID, Budget: &workflow.Budget{ModelCalls: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, ns, item.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	return WithRun(ctx, run, "agent", uuid.NewString(), 1), run
}

func TestHTTPBudgetBlocksNetworkAndPreservesProviderUsage(t *testing.T) {
	ctx, run := httpScope(t)
	var calls atomic.Int32
	raw := `{"choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":500,"prompt_tokens_details":{"cached_tokens":100}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, raw) }))
	defer server.Close()
	client := http.Client{Transport: &Transport{Kind: "model", Model: "test", Price: &workflow.Price{Currency: "USD", InputPerMillionMicros: 2000000, OutputPerMillionMicros: 6000000}}}
	req, err := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != raw {
		t.Fatal("响应正文被用量层破坏")
	}
	if _, err = client.Do(req); workflow.FailureKind(err) != "budget" {
		t.Fatalf("超额错误 %v", err)
	}
	if calls.Load() != 1 {
		t.Fatal("超额调用到达提供方")
	}
	report, err := ForRun(ctx, run)
	if err != nil || len(report.Calls) != 1 {
		t.Fatalf("用量记录 %+v %v", report, err)
	}
	call := report.Calls[0]
	if call.InputTokens == nil || *call.InputTokens != 1000 || call.OutputTokens == nil || *call.OutputTokens != 500 || call.CachedTokens == nil || *call.CachedTokens != 100 || call.EstimateMicros == nil || *call.EstimateMicros != 5000 {
		t.Fatalf("提供方用量丢失 %+v", call)
	}
}
