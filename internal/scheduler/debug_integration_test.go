package scheduler_test

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"os"
	"testing"
	"time"
)

func TestHatchetDebugStepAndResumeAtNodeBoundary(t *testing.T) {
	_, client := integrationClient(t)
	started := make(chan string, 3)
	release := make(chan struct{})
	worker, err := scheduler.NewWorker(client, func(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		if input.Node.ID == "seed" {
			debugSQL(t, "INSERT INTO pangu.run_debug(run_id,paused) VALUES($1,true)", input.RunID)
		}
		started <- input.Node.ID
		if input.Node.ID == "a" {
			select {
			case <-release:
			case <-ctx.Done():
				return workflow.NodeOutput{}, ctx.Err()
			}
		}
		return workflow.NodeOutput{Text: "完成"}, nil
	}, mockPreflight)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	b := scheduler.NewBackend(client)
	awaitWorker(t, b)
	project := registerTestProject(t, b)
	nodes := []workflow.Node{{ID: "seed", Executor: "eino", Model: "test", Handoff: testHandoff(), Instruction: "准备"}}
	for _, id := range []string{"a", "b"} {
		nodes = append(nodes, workflow.Node{ID: id, Executor: "eino", Model: "test", Handoff: testHandoff(), Instruction: "单步", DependsOn: []string{"seed"}})
	}
	record, err := b.Submit(context.Background(), workflow.Definition{ProjectID: project.ID, Nodes: nodes}, "debug-step")
	if err != nil {
		t.Fatal(err)
	}
	awaitDebugNode(t, started, "seed")
	awaitDebugIdle(t, record.RunID)
	select {
	case node := <-started:
		t.Fatalf("暂停后启动 %s", node)
	case <-time.After(1200 * time.Millisecond):
	}
	debugSQL(t, "UPDATE pangu.run_debug SET step_requested=true,revision=revision+1 WHERE run_id=$1", record.RunID)
	awaitDebugNode(t, started, "a")
	select {
	case node := <-started:
		t.Fatalf("单步释放多个节点: %s", node)
	case <-time.After(1200 * time.Millisecond):
	}
	close(release)
	awaitDebugIdle(t, record.RunID)
	state, err := execution.GetDebug(context.Background(), record.RunID)
	if err != nil || !state.Paused || state.StepRequested || state.StepNode != "" {
		t.Fatalf("未保持暂停: %+v %v", state, err)
	}
	debugSQL(t, "UPDATE pangu.run_debug SET paused=false,revision=revision+1 WHERE run_id=$1", record.RunID)
	awaitDebugNode(t, started, "b")
	result := waitRun(t, api.New(b, integrationAccess{}), record.RunID)
	if result.Status != "succeeded" {
		t.Fatalf("继续失败: %+v", result)
	}
	events, err := trace.List(context.Background(), record.RunID, trace.Query{Limit: 100, Kind: "node"})
	if err != nil || len(events.Items) != 6 {
		t.Fatalf("节点 trace 缺失: %+v %v", events, err)
	}
}

func debugSQL(t *testing.T, sql, run string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, sql, run); err != nil {
		t.Fatal(err)
	}
}
func awaitDebugNode(t *testing.T, ch <-chan string, want string) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("期望 %s，启动 %s", want, got)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("节点 %s 未启动", want)
	}
}
func awaitDebugIdle(t *testing.T, run string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		state, err := execution.GetDebug(context.Background(), run)
		if err == nil && state.Paused && state.Active == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("暂停节点未收敛")
}

func TestHatchetPausedRetrySurvivesWorkerRestart(t *testing.T) {
	if os.Getenv("PANGU_TEST_BINARY") == "" {
		t.Skip("需要构建后的 Worker")
	}
	model, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	project := registerTestProject(t, backend)
	process := startProcess(t)
	awaitWorker(t, backend)
	definition := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{
		{ID: "source", Executor: "eino", Model: "source", Handoff: testHandoff(), Instruction: "准备"},
		{ID: "recover", Executor: "eino", Model: "recover", Handoff: testHandoff(), Instruction: "恢复", DependsOn: []string{"source"}},
		{ID: "final", Executor: "eino", Model: "final", Handoff: testHandoff(), Instruction: "收尾", DependsOn: []string{"recover"}},
	}}
	record, err := backend.Submit(context.Background(), definition, "paused-restart")
	if err != nil {
		t.Fatal(err)
	}
	awaitDebugModel(t, model, "recover", 1)
	debugSQL(t, "INSERT INTO pangu.run_debug(run_id,paused) VALUES($1,true)", record.RunID)
	if err = process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = process.Wait()
	startProcess(t)
	awaitDebugIdle(t, record.RunID)
	if model.count("recover") != 1 || model.count("final") != 0 {
		t.Fatal("重启绕过暂停状态")
	}
	debugSQL(t, "UPDATE pangu.run_debug SET step_requested=true,revision=revision+1 WHERE run_id=$1", record.RunID)
	awaitDebugModel(t, model, "recover", 2)
	awaitDebugIdle(t, record.RunID)
	if model.count("final") != 0 {
		t.Fatal("重试消费单步许可后仍启动后续节点")
	}
	debugSQL(t, "UPDATE pangu.run_debug SET paused=false,revision=revision+1 WHERE run_id=$1", record.RunID)
	result := waitRun(t, api.New(backend, integrationAccess{}), record.RunID)
	if result.Status != "succeeded" || model.count("source") != 1 || model.count("recover") != 2 || model.count("final") != 1 {
		t.Fatalf("暂停恢复重复执行或丢失节点: %+v", result)
	}
}

func awaitDebugModel(t *testing.T, model *modelServer, name string, count int) {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if model.count(name) == count {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("模型 %s 未达到 %d 次调用，实际 %d", name, count, model.count(name))
}
