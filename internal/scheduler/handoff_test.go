package scheduler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func testHandoff() *workflow.HandoffContract {
	return &workflow.HandoffContract{Objective: "完成指定任务", Inputs: "本次输入和上游产物", Deliverables: "任务结果", Acceptance: "结果符合本次要求且说明验证证据", FailureHandling: "明确缺口，不得伪造完成"}
}

func TestHatchetHandoffBlocksNewRequestsButPreservesAcceptedReceipt(t *testing.T) {
	m, backend, h := integrationSetup(t)
	project := registerTestProject(t, backend)
	d := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "old", Instruction: "升级前已接受"}}}
	ctx := context.Background()
	old, err := submission.Prepare(ctx, backend.Namespace(), "submit", "old-key", submission.Snapshot{Workflow: d})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(d)
	post := func(key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/runs", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	first := post("old-key")
	var receipt submission.Record
	if first.Code != 202 || json.Unmarshal(first.Body.Bytes(), &receipt) != nil || receipt.ID != old.ID {
		t.Fatalf("旧回执不可重取: %d %s", first.Code, first.Body.String())
	}
	_ = waitRun(t, h, receipt.RunID)
	if again := post("old-key"); again.Code != 202 || m.count("old") != 1 {
		t.Fatal("旧键重试重复执行或被拒绝", again.Code)
	}
	if fresh := post("new-key"); fresh.Code != 400 || !strings.Contains(fresh.Body.String(), "交接契约") {
		t.Fatalf("新键绕过契约: %d %s", fresh.Code, fresh.Body.String())
	}
	if _, err := backend.Rerun(ctx, receipt.RunID, "new-rerun"); err == nil || !strings.Contains(err.Error(), "交接契约") {
		t.Fatalf("重跑绕过契约: %v", err)
	}
	if m.count("old") != 1 {
		t.Fatal("无效的新任务调用了模型")
	}
}

func TestHatchetHandoffRequiredForFork(t *testing.T) {
	backend, _ := forkWorker(t)
	p := registerTestProject(t, backend)
	ctx := context.Background()
	d := workflow.Definition{ProjectID: p.ID, Nodes: []workflow.Node{{ID: "build", Executor: "codex", Instruction: "实现", Handoff: testHandoff()}}}
	first, err := backend.Submit(ctx, d, "source")
	if err != nil {
		t.Fatal(err)
	}
	_ = waitRun(t, api.New(backend, integrationAccess{}), first.RunID)
	d.Nodes[0].Handoff = nil
	if _, err := backend.Fork(ctx, first.RunID, "new-fork", workflow.ForkInput{CheckpointNode: "build", Workflow: d}); err == nil || !strings.Contains(err.Error(), "交接契约") {
		t.Fatalf("返工绕过契约: %v", err)
	}
}

func TestHatchetHandoffCronSkipsInvalidSnapshotAndReleasesSlot(t *testing.T) {
	_, backend, h := integrationSetup(t)
	p := registerTestProject(t, backend)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cron := workflow.Cron{Name: "handoff-cron", Expression: "0 0 1 1 *", Workflow: workflow.Definition{ProjectID: p.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "cron", Instruction: "定时任务"}}}}
	if _, err := backend.CreateCron(ctx, cron); err == nil {
		t.Fatal("新计划绕过契约")
	}
	old, err := schedule.Create(ctx, backend.Namespace(), cron)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.DeleteCron(context.Background(), old.ID) })
	slot := time.Now().UTC().Truncate(time.Minute)
	active := func(context.Context, string) (bool, error) { return false, nil }
	if _, err := schedule.Claim(ctx, backend.Namespace(), old.ID, old.Revision, slot, active); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { backend.Maintain(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	fire := waitHandoffFire(t, backend.Namespace(), old.ID, "skipped")
	if fire.RequestID != "" || !strings.Contains(fire.Reason, "交接契约") {
		t.Fatalf("旧计划未说明跳过原因: %+v", fire)
	}
	cron.Workflow.Nodes[0].Handoff = testHandoff()
	next, err := backend.UpdateCron(ctx, old.ID, old.Revision, cron, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schedule.Claim(ctx, backend.Namespace(), next.ID, next.Revision, slot.Add(time.Minute), active); err != nil {
		t.Fatal(err)
	}
	fire = waitHandoffFire(t, backend.Namespace(), old.ID, "accepted")
	receipt, err := backend.GetRequest(ctx, fire.RequestID)
	for deadline := time.Now().Add(10 * time.Second); err == nil && receipt.RunID == "" && time.Now().Before(deadline); {
		time.Sleep(100 * time.Millisecond)
		receipt, err = backend.GetRequest(ctx, fire.RequestID)
	}
	if err != nil || receipt.RunID == "" {
		t.Fatalf("有效计划无法继续: %+v %v", receipt, err)
	}
	_ = waitRun(t, h, receipt.RunID)
}

func waitHandoffFire(t *testing.T, namespace, id, status string) schedule.Fire {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		fires, err := schedule.Fires(context.Background(), namespace, id, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(fires) > 0 && fires[0].Status == status {
			return fires[0]
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("计划未进入 %s", status)
	return schedule.Fire{}
}
