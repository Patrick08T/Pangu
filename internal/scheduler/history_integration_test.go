package scheduler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func TestHatchetHistorySurvivesRemoteExpiry(t *testing.T) {
	_, b, h := integrationSetup(t)
	project := registerTestProject(t, b)
	ctx := context.Background()
	d := workflow.Definition{ProjectID: project.ID, Input: "临时私密输入", Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "one", Handoff: testHandoff(), Instruction: "临时指令"}}}
	receipt, err := b.Submit(ctx, d, "history")
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, h, receipt.RunID)
	if err := b.ArchiveRuns(ctx); err != nil {
		t.Fatal(err)
	}
	ageHistory(t, receipt.RunID)
	remote, _ := url.Parse(os.Getenv("HATCHET_CLIENT_SERVER_URL"))
	proxy := httputil.NewSingleHostReverseProxy(remote)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, receipt.RunID) {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer server.Close()
	t.Setenv("HATCHET_CLIENT_SERVER_URL", server.URL)
	details, err := b.GetRun(ctx, receipt.RunID)
	if err != nil || !details.StopConfirmed || !details.LogsExpired || details.Status != "succeeded" || len(details.Nodes) != 1 {
		t.Fatalf("远端到期后历史丢失: %+v %v", details, err)
	}
	encoded, _ := json.Marshal(details)
	if details.Definition != nil || len(details.Events) != 0 || len(details.Attempts) != 0 || details.Nodes[0].Output != "" || strings.Contains(string(encoded), "临时") {
		t.Fatalf("归档包含正文: %s", encoded)
	}
	q := scheduler.RunQuery{ProjectID: project.ID, Since: time.Now().Add(-90 * 24 * time.Hour), Until: time.Now(), Limit: 1}
	page, err := b.ListRuns(ctx, q)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != receipt.RunID || page.NextOffset != nil {
		t.Fatalf("到期后列表: %+v %v", page, err)
	}
	q.Offset = 1
	page, err = b.ListRuns(ctx, q)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("归档分页重复: %+v %v", page, err)
	}
	if _, err := b.Events(ctx, receipt.RunID, "", 0, 20); !errors.Is(err, workspace.ErrExpired) {
		t.Fatal("事件到期未返回410", err)
	}
	deleteHistory(t, receipt.RunID)
	q.Offset = 0
	page, err = b.ListRuns(ctx, q)
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != "unknown" {
		t.Fatalf("单条状态不可用破坏整页: %+v %v", page, err)
	}
}

func deleteHistory(t *testing.T, id string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "DELETE FROM pangu.run_history WHERE run_id=$1", id); err != nil {
		t.Fatal(err)
	}
}

func ageHistory(t *testing.T, id string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var body []byte
	if err := conn.QueryRow(ctx, "SELECT summary FROM pangu.run_history WHERE run_id=$1", id).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "临时") || strings.Contains(string(body), "instruction") {
		t.Fatalf("归档保存了正文: %s", body)
	}
	var run scheduler.RunDetails
	if err := json.Unmarshal(body, &run); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-29 * 24 * time.Hour)
	run.CreatedAt = time.Now().Add(-31 * 24 * time.Hour)
	run.FinishedAt = &old
	body, err = json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "UPDATE pangu.run_history SET summary=$2,finished_at=$3 WHERE run_id=$1", id, body, old); err != nil {
		t.Fatal(err)
	}
}
