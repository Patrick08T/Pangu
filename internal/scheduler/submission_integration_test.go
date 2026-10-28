package scheduler_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func TestHatchetRerunUsesSavedBaseWhenSourceIsUnavailable(t *testing.T) {
	m, backend, h := integrationSetup(t)
	project := registerTestProject(t, backend)
	d := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "rerun", Handoff: testHandoff(), Instruction: "重跑原快照"}}}
	ctx := context.Background()
	first, err := backend.Submit(ctx, d, "original")
	if err != nil || first.RunID == "" {
		t.Fatalf("提交: %+v %v", first, err)
	}
	_ = waitRun(t, h, first.RunID)
	// 临时测试源项目改名，确认重跑只依赖已保留的原基准。
	moved := filepath.Join(filepath.Dir(project.Source), "moved-source")
	if err := os.Rename(project.Source, moved); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(moved, project.Source) })
	second, err := backend.Rerun(ctx, first.RunID, "rerun-key")
	if err != nil || second.RunID == "" || second.RunID == first.RunID {
		t.Fatalf("重跑: %+v %v", second, err)
	}
	_ = waitRun(t, h, second.RunID)
	again, err := backend.Rerun(ctx, first.RunID, "rerun-key")
	if err != nil || again.RunID != second.RunID || m.count("rerun") != 2 {
		t.Fatalf("重跑防重失败: %+v %v", again, err)
	}
	a, err := workspace.Get(ctx, first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := workspace.Get(ctx, second.RunID)
	if err != nil || a.BaseCommit != b.BaseCommit {
		t.Fatalf("基准变化: %+v %v", b, err)
	}
}

func TestHatchetPersistentSubmissionSurvivesLostStoredReceipt(t *testing.T) {
	m, backend, h := integrationSetup(t)
	project := registerTestProject(t, backend)
	d := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "once", Handoff: testHandoff(), Instruction: "只执行一次"}}}
	ctx := context.Background()
	items := make(chan *submission.Record, 4)
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			item, err := backend.Submit(ctx, d, "same-client-request")
			if err != nil {
				t.Errorf("提交失败: %v", err)
				return
			}
			items <- item
		})
	}
	group.Wait()
	close(items)
	var first *submission.Record
	for item := range items {
		if first != nil && (first.ID != item.ID || first.RunID != item.RunID) {
			t.Fatal("同键创建多个请求或运行")
		}
		first = item
	}
	if first == nil || first.RunID == "" {
		t.Fatal("请求未交接")
	}
	_ = waitRun(t, h, first.RunID)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 故障注入：调度已接受，但 API 未持久保存回执的数据库状态。
	if _, err := conn.Exec(ctx, "DELETE FROM pangu.run_history WHERE run_id=$1", first.RunID); err != nil {
		_ = conn.Close(ctx)
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.submissions SET run_id=NULL WHERE id=$1", first.ID)
	_ = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	other := backend
	again, err := other.Submit(ctx, d, "same-client-request")
	if err != nil || again.RunID != first.RunID || m.count("once") != 1 {
		t.Fatalf("回执丢失导致重复执行: %+v %v count=%d", again, err, m.count("once"))
	}
	d.Input = "changed"
	if _, err := other.Submit(ctx, d, "same-client-request"); !errors.Is(err, submission.ErrConflict) {
		t.Fatalf("同键异内容没有冲突: %v", err)
	}
	t.Logf("持久防重和丢失回执对账通过 request=%s run=%s", first.ID, first.RunID)
}

func TestHatchetRegisteredButOfflineWorkerRejectsNewSubmission(t *testing.T) {
	_, client := integrationClient(t)
	worker, err := scheduler.NewWorker(client, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "未执行"}, nil
	}, mockPreflight)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	backend := scheduler.NewBackend(client)
	awaitWorker(t, backend)
	project := registerTestProject(t, backend)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	d := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "offline", Handoff: testHandoff(), Instruction: "拒绝"}}}
	if item, err := backend.Submit(context.Background(), d, "offline"); err == nil || item != nil {
		t.Fatalf("离线 Worker 接受了新提交: %+v %v", item, err)
	}
}
