package scheduler_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetRerunRejectsInvalidHistoricalApprovalBeforeSubmission(t *testing.T) {
	m, backend, handler := integrationSetup(t)
	project := registerTestProject(t, backend)
	ctx := context.Background()
	d := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "build", Executor: "eino", Model: "original", Instruction: "原任务", Handoff: testHandoff()}}}
	first, err := backend.Submit(ctx, d, "source")
	if err != nil {
		t.Fatal(err)
	}
	_ = waitRun(t, handler, first.RunID)
	previous, err := submission.ForRun(ctx, backend.Namespace(), first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	// 仅在独立测试库模拟升级前已保存的空审批配置。
	previous.Snapshot.Workflow.Nodes[0].Approval = &workflow.Approval{}
	raw, err := json.Marshal(previous.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.submissions SET snapshot=$2,payload_hash=$3 WHERE id=$1", first.ID, raw, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/runs/"+first.RunID+"/rerun", nil)
	r.Header.Set("Idempotency-Key", "invalid-legacy-rerun")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "source_node") || !strings.Contains(w.Body.String(), "新版") {
		t.Fatalf("历史无效审批未在提交前明确拒绝: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err = conn.QueryRow(ctx, "SELECT count(*) FROM pangu.submissions WHERE namespace=$1 AND operation='rerun'", backend.Namespace()).Scan(&count); err != nil || count != 0 || m.count("original") != 1 {
		t.Fatalf("无效重跑产生请求或执行: count=%d err=%v", count, err)
	}
}
