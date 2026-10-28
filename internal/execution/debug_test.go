package execution

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/google/uuid"
)

func debugDatabase(t *testing.T) (context.Context, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return ctx, uuid.NewString()
}

func TestStoppedAttemptClosesUnfinishedTraceWithoutInventingResults(t *testing.T) {
	ctx, run := debugDatabase(t)
	item := Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: "writer"}
	if err := Begin(ctx, &item); err != nil {
		t.Fatal(err)
	}
	ctx = trace.WithRun(ctx, run, item.NodeID, item.NodeRunID, item.Attempt)
	if _, _, err := trace.Start(ctx, "tool", "exec_command", "pending"); err != nil {
		t.Fatal(err)
	}
	item.Status = "cancelled"
	if err := Finish(ctx, item); err != nil {
		t.Fatal(err)
	}
	page, err := trace.List(ctx, run, trace.Query{Limit: 10})
	if err != nil || len(page.Items) != 2 || page.Items[0].Phase != "end" || page.Items[0].Status != "cancelled" || page.Items[0].Output != "" || page.Items[0].Error == "" {
		t.Fatalf("中断被误报为成功或仍在执行: %+v %v", page, err)
	}
}

func TestPausedRunReleasesExactlyOneNodeAndPersists(t *testing.T) {
	ctx, run := debugDatabase(t)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "INSERT INTO pangu.run_debug(run_id,paused,step_requested) VALUES($1,true,false)", run)
	if err != nil {
		t.Fatal(err)
	}
	item := Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: "a"}
	if err = Begin(ctx, &item); !errors.Is(err, ErrPaused) {
		t.Fatalf("暂停节点已启动: %v", err)
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.run_debug SET step_requested=true WHERE run_id=$1", run)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var started []Attempt
	var wg sync.WaitGroup
	for _, node := range []string{"a", "b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			candidate := Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: node}
			err := Begin(ctx, &candidate)
			if err == nil {
				mu.Lock()
				started = append(started, candidate)
				mu.Unlock()
			} else if !errors.Is(err, ErrPaused) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(started) != 1 {
		t.Fatalf("单步启动 %d 个节点", len(started))
	}
	started[0].Status = "succeeded"
	if err = Finish(ctx, started[0]); err != nil {
		t.Fatal(err)
	}
	if err = Begin(ctx, &item); !errors.Is(err, ErrPaused) {
		t.Fatalf("许可被重复使用: %v", err)
	}
	state, err := GetDebug(ctx, run)
	if err != nil || !state.Paused || state.StepRequested || state.StepNode != "" {
		t.Fatalf("状态未持久化: %+v %v", state, err)
	}
	if err = RequestCancel(ctx, "test", run); err != nil {
		t.Fatal(err)
	}
	if _, err = DispatchReady(ctx, run, "a"); !errors.Is(err, ErrCancelled) {
		t.Fatalf("暂停时取消无效: %v", err)
	}
}
