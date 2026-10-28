package scheduler_test

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func forkWorker(t *testing.T) (scheduler.Backend, func(string) int) {
	_, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	var lock sync.Mutex
	calls := map[string]int{}
	worker, err := scheduler.NewWorker(client, func(_ context.Context, in workflow.NodeInput) (workflow.NodeOutput, error) {
		lock.Lock()
		calls[in.Node.ID]++
		lock.Unlock()
		if in.Node.Instruction == "fail" {
			return workflow.NodeOutput{}, &workflow.Failure{Kind: "validation", Cause: errors.New("需要返工")}
		}
		if in.Node.ReadOnly {
			return workflow.NodeOutput{Text: "已只读核验"}, nil
		}
		name := filepath.Join(in.Workspace, in.Node.ID+".txt")
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			return workflow.NodeOutput{}, errors.New("重复执行在已有产物上")
		}
		if err := os.WriteFile(name, []byte(in.Node.Instruction), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{Text: in.Node.Instruction}, nil
	}, func(context.Context, workflow.Definition) workflow.Preflight { return workflow.Preflight{Ready: true} })
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })
	awaitWorker(t, backend)
	return backend, func(node string) int { lock.Lock(); defer lock.Unlock(); return calls[node] }
}

func TestHatchetForkReusesCheckpointAndRerunStartsFromOrigin(t *testing.T) {
	backend, count := forkWorker(t)
	p := registerTestProject(t, backend)
	d := workflow.Definition{ProjectID: p.ID, Input: "任务", Nodes: []workflow.Node{{ID: "build", Executor: "codex", Handoff: testHandoff(), Instruction: "实现"}, {ID: "review", Executor: "codex", Handoff: testHandoff(), Instruction: "fail", DependsOn: []string{"build"}}}}
	ctx := context.Background()
	first, err := backend.Submit(ctx, d, "initial")
	if err != nil {
		t.Fatal(err)
	}
	old := waitFailedRun(t, backend, first.RunID)
	if !old.StopConfirmed {
		t.Fatal("来源未停止")
	}
	d.Nodes[1].Instruction = "修复"
	input := workflow.ForkInput{CheckpointNode: "build", Workflow: d}
	fork := awaitFork(t, backend, strings.ToUpper(first.RunID), input)
	again, err := backend.Fork(ctx, strings.ToUpper(first.RunID), "fork", input)
	if err != nil || again.ID != fork.ID {
		t.Fatalf("大写路径重复提交: %+v %v", again, err)
	}
	h := api.New(backend, integrationAccess{})
	out := waitRun(t, h, fork.RunID)
	if len(out.Nodes) != 2 || out.Fork == nil || len(out.Fork.Reused) != 1 || out.SourceRunID != first.RunID {
		t.Fatalf("返工关系 %+v", out)
	}
	if count("build") != 1 {
		t.Fatal("重复执行实现节点")
	}
	assertForkDescendants(t, backend, fork.RunID, d, count)
	changed := input
	changed.Workflow.Input = "不同输入"
	if _, err = backend.Fork(ctx, first.RunID, "fork", changed); !errors.Is(err, submission.ErrConflict) {
		t.Fatalf("重复键接受变更 %v", err)
	}
	original, err := workspace.Get(ctx, first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "cat-file", "-e", original.HeadCommit+":review.txt")
	command.Dir = filepath.Join(original.Directory, "base")
	if command.Run() == nil {
		t.Fatal("旧运行被返工覆盖")
	}
	assertForkReceiptAfterSourceRemoval(t, backend, first.RunID, fork.ID, input)
}

func assertForkReceiptAfterSourceRemoval(t *testing.T, b scheduler.Backend, source, receipt string, input workflow.ForkInput) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "DELETE FROM pangu.submissions WHERE run_id=$1", source); err != nil {
		t.Fatal(err)
	}
	item, err := b.Fork(ctx, source, "fork", input)
	if err != nil || item.ID != receipt {
		t.Fatalf("来源已清理但返工回执仍应可找回: %+v %v", item, err)
	}
}

func assertForkDescendants(t *testing.T, backend scheduler.Backend, run string, d workflow.Definition, count func(string) int) {
	t.Helper()
	ctx := context.Background()
	h := api.New(backend, integrationAccess{})
	next, err := backend.Fork(ctx, run, "fork-again", workflow.ForkInput{CheckpointNode: "review", Workflow: d})
	if err != nil {
		t.Fatal(err)
	}
	out := waitRun(t, h, next.RunID)
	if len(out.Fork.Reused) != 2 {
		t.Fatal("再次返工丢失检查点")
	}
	redo, err := backend.Rerun(ctx, next.RunID, "redo")
	if err != nil {
		t.Fatal(err)
	}
	_ = waitRun(t, h, redo.RunID)
	if count("build") != 2 {
		t.Fatal("整次重跑未执行前缀")
	}
}

func awaitFork(t *testing.T, backend scheduler.Backend, source string, input workflow.ForkInput) *submission.Record {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		item, err := backend.Fork(context.Background(), source, "fork", input)
		if err == nil {
			return item
		}
		if !errors.Is(err, scheduler.ErrActive) && !errors.Is(err, scheduler.ErrNotReady) {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("根任务重试结束后仍不能返工")
	return nil
}
