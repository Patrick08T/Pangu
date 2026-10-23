package submission

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func submissionProject(t *testing.T) string {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	id := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,'submission-test','幂等测试','unused','main','test')`, id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPersistentRequestRejectsChangedPayloadAndSerializesDispatch(t *testing.T) {
	project := submissionProject(t)
	ctx := context.Background()
	d := workflow.Definition{ProjectID: project, Input: "原始输入"}
	first, err := Prepare(ctx, "submission-test", "submit", "same-key", Snapshot{Workflow: d})
	if err != nil {
		t.Fatal(err)
	}
	again, err := Prepare(ctx, "submission-test", "submit", "same-key", Snapshot{Workflow: d})
	if err != nil || again.ID != first.ID {
		t.Fatalf("没有稳定请求 ID: %+v %v", again, err)
	}
	d.Input = "改变输入"
	if _, err := Prepare(ctx, "submission-test", "submit", "same-key", Snapshot{Workflow: d}); !errors.Is(err, ErrConflict) {
		t.Fatalf("没有拒绝同键不同内容: %v", err)
	}
	var calls atomic.Int32
	run := uuid.NewString()
	send := func(context.Context, *Record) (string, error) { calls.Add(1); return run, nil }
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			out, err := Dispatch(ctx, "submission-test", first.ID, send)
			if err != nil || out.RunID != run {
				t.Errorf("交接失败: %+v %v", out, err)
			}
		})
	}
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("重复派发: %d", calls.Load())
	}
	if _, err := Get(ctx, "other-namespace", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨命名空间读取请求: %v", err)
	}
}

func TestUnknownDeliveryRemainsPendingForReconciliation(t *testing.T) {
	project := submissionProject(t)
	ctx := context.Background()
	request, err := Prepare(ctx, "submission-test", "submit", "lost-ack", Snapshot{Workflow: workflow.Definition{ProjectID: project}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Dispatch(ctx, "submission-test", request.ID, func(context.Context, *Record) (string, error) { return "", errors.New("token=do-not-expose") })
	if err == nil {
		t.Fatal("未报告未知派发结果")
	}
	stored, err := Get(ctx, "submission-test", request.ID)
	if err != nil || stored.Status != "pending" || stored.RunID != "" {
		t.Fatalf("未知结果被错误确认: %+v %v", stored, err)
	}
	result, err := Dispatch(ctx, "submission-test", request.ID, func(context.Context, *Record) (string, error) { return uuid.NewString(), nil })
	if err != nil || result.Status != "accepted" {
		t.Fatalf("无法对账: %+v %v", result, err)
	}
}
