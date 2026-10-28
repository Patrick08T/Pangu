package schedule

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func schedulePlan(t *testing.T) *Plan {
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
	project := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,'schedule-test','计划测试','unused','main','test')`, project)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Create(ctx, "schedule-test", workflow.Cron{Name: "test-" + uuid.NewString(), Expression: "* * * * *", Workflow: workflow.Definition{ProjectID: project, Nodes: []workflow.Node{{ID: "test", Executor: "eino", Model: "test", Instruction: "计划"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestScheduleConcurrentClaimKeepsOneAcceptedSlot(t *testing.T) {
	plan := schedulePlan(t)
	ctx := context.Background()
	slot := time.Now().UTC().Truncate(time.Minute)
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			fire, err := Claim(ctx, "schedule-test", plan.ID, plan.Revision, slot, func(context.Context, string) (bool, error) { return false, nil })
			if err != nil || fire.Status != "pending" || !fire.Slot.Equal(slot) {
				t.Errorf("重复触发状态错误: %+v %v", fire, err)
			}
		})
	}
	group.Wait()
	fires, err := Fires(ctx, "schedule-test", plan.ID, 0)
	if err != nil || len(fires) != 1 {
		t.Fatalf("同分钟重复创建触发记录: %d %v", len(fires), err)
	}
}

func TestScheduleRecordsOverlapAndDoesNotCatchUpMissedSlots(t *testing.T) {
	plan := schedulePlan(t)
	ctx := context.Background()
	slot := time.Now().UTC().Truncate(time.Minute)
	first, err := Claim(ctx, "schedule-test", plan.ID, plan.Revision, slot, func(context.Context, string) (bool, error) { return false, nil })
	if err != nil {
		t.Fatal(err)
	}
	request, err := submission.Prepare(ctx, "schedule-test", "cron", "slot", first.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := Bind(ctx, first, request.ID); err != nil {
		t.Fatal(err)
	}
	next, err := Claim(ctx, "schedule-test", plan.ID, plan.Revision, slot.Add(time.Minute), func(context.Context, string) (bool, error) { return true, nil })
	if err != nil || next.Status != "skipped" || next.Reason != "overlap" {
		t.Fatalf("没有跳过活动运行: %+v %v", next, err)
	}
	missed, err := Claim(ctx, "schedule-test", plan.ID, plan.Revision, slot.Add(-2*time.Minute), func(context.Context, string) (bool, error) { return false, nil })
	if err != nil || missed.Status != "skipped" || missed.Reason != "missed" {
		t.Fatalf("补跑了遗漏时间: %+v %v", missed, err)
	}
	if err := Delete(ctx, "schedule-test", plan.ID); err != nil {
		t.Fatal(err)
	}
	deleted, err := Claim(ctx, "schedule-test", plan.ID, plan.Revision, slot.Add(2*time.Minute), func(context.Context, string) (bool, error) { return false, nil })
	if err != nil || deleted.Reason != "deleted" {
		t.Fatalf("删除后仍派生任务: %+v %v", deleted, err)
	}
}
