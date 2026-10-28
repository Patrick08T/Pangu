package scheduler_test

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetCronLostReceiptIsReconciledAndDeleted(t *testing.T) {
	_, backend, _ := integrationSetup(t)
	project := registerTestProject(t, backend)
	ctx := context.Background()
	c := workflow.Cron{Name: "receipt-test", Expression: "0 0 1 1 *", Workflow: workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "cron", Handoff: testHandoff(), Instruction: "测试"}}}}
	plan, err := backend.CreateCron(ctx, c)
	if err != nil || !plan.Ready {
		t.Fatalf("创建: %+v %v", plan, err)
	}
	t.Cleanup(func() { _ = backend.DeleteCron(ctx, plan.ID) })
	again, err := backend.CreateCron(ctx, c)
	if err != nil || again.ID != plan.ID {
		t.Fatalf("同名同配置重试创建重复计划: %+v %v", again, err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, deleting := range []bool{false, true} {
		if _, err := conn.Exec(ctx, "UPDATE pangu.schedules SET hatchet_id=NULL,remote_synced=false WHERE id=$1", plan.ID); err != nil {
			t.Fatal(err)
		}
		if deleting {
			err = backend.DeleteCron(ctx, plan.ID)
		} else {
			err = backend.ReconcileCrons(ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		stored, err := schedule.Get(ctx, backend.Namespace(), plan.ID)
		if err != nil || !stored.Ready || (!deleting && stored.HatchetID != plan.HatchetID) || (deleting && stored.HatchetID != "") {
			t.Fatalf("未知回执对账失败: %+v %v", stored, err)
		}
	}
}

func TestHatchetCronPauseEditResumeUsesPersistedRevision(t *testing.T) {
	_, backend, _ := integrationSetup(t)
	project := registerTestProject(t, backend)
	ctx := context.Background()
	c := workflow.Cron{Name: "edit-test", Expression: "0 0 1 1 *", Workflow: workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "cron", Handoff: testHandoff(), Instruction: "测试"}}}}
	plan, err := backend.CreateCron(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.DeleteCron(ctx, plan.ID) })
	paused, err := backend.UpdateCron(ctx, plan.ID, plan.Revision, c, true)
	if err != nil || !paused.Ready || !paused.Paused || paused.HatchetID != "" {
		t.Fatalf("暂停未落地: %+v %v", paused, err)
	}
	again, err := backend.UpdateCron(ctx, plan.ID, plan.Revision, c, true)
	if err != nil || again.Revision != paused.Revision {
		t.Fatalf("暂停重试未幂等: %+v %v", again, err)
	}
	c.Expression = "0 0 2 1 *"
	resumed, err := backend.UpdateCron(ctx, plan.ID, paused.Revision, c, false)
	if err != nil || !resumed.Ready || resumed.Paused || resumed.HatchetID == "" || resumed.HatchetID == plan.HatchetID {
		t.Fatalf("恢复未采用新版本: %+v %v", resumed, err)
	}
	if _, err := backend.UpdateCron(ctx, plan.ID, plan.Revision, c, false); err == nil {
		t.Fatal("旧修订覆盖新计划")
	}
}
