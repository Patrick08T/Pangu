package scheduler_test

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

func approvalActor(t *testing.T, namespace string) identity.Actor {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	actor := identity.Actor{ID: uuid.NewString(), Namespace: namespace}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, actor.ID, namespace)
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func awaitApproval(t *testing.T, namespace, run string) *approval.Gate {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		gates, err := approval.List(context.Background(), namespace, run)
		if err != nil {
			t.Fatal(err)
		}
		if len(gates) > 0 {
			return &gates[0]
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("未生成审批")
	return nil
}

func TestHatchetApprovalWaitsUntilDecision(t *testing.T) {
	model, backend, _ := approvalIntegrationSetup(t)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	d := approvalWorkflow(project.ID, "waiting")
	submitted, err := backend.Submit(ctx, d, "approval-wait")
	if err != nil || submitted.RunID == "" {
		t.Fatalf("提交: %+v %v", submitted, err)
	}
	gate := awaitApproval(t, backend.Namespace(), submitted.RunID)
	out, err := backend.GetRun(ctx, submitted.RunID)
	if err != nil || out.Status != "waiting_approval" || out.StopConfirmed || model.count("waiting") != 0 {
		t.Fatalf("未进入待审批状态: %+v %v calls=%d", out, err, model.count("waiting"))
	}
	q := scheduler.RunQuery{ProjectID: project.ID, Status: "waiting_approval", Limit: 20, Since: time.Now().Add(-time.Hour), Until: time.Now()}
	items, err := backend.ListRuns(ctx, q)
	if err != nil || len(items.Items) != 1 || items.Items[0].ID != submitted.RunID {
		t.Fatalf("待审批列表缺少运行: %+v %v", items, err)
	}
	if _, err = approval.Decide(ctx, actor, gate.ID, approval.Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}}}); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	finished := waitRun(t, api.New(backend, integrationAccess{}), submitted.RunID)
	if finished.Status != "succeeded" || model.count("waiting") != 1 {
		t.Fatalf("批准后没有正确恢复: %s calls=%d", finished.Status, model.count("waiting"))
	}
}

func TestHatchetApprovalRejectionStopsBeforeModel(t *testing.T) {
	model, backend, _ := approvalIntegrationSetup(t)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	d := approvalWorkflow(project.ID, "rejected")
	submitted, err := backend.Submit(ctx, d, "approval-rejected")
	if err != nil {
		t.Fatal(err)
	}
	gate := awaitApproval(t, backend.Namespace(), submitted.RunID)
	if _, err = approval.Decide(ctx, actor, gate.ID, approval.Decision{SubjectHash: gate.SubjectHash, Decision: "rejected", Reason: "范围不符合要求"}); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	out := waitFailedRun(t, backend, submitted.RunID)
	if model.count("rejected") != 0 || out.Error == nil || !strings.Contains(out.Error.Message, "rejected") {
		t.Fatalf("驳回后仍执行或原因不明: calls=%d error=%+v", model.count("rejected"), out.Error)
	}
}

func TestHatchetWaitingApprovalCanBeCancelled(t *testing.T) {
	model, backend, _ := approvalIntegrationSetup(t)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	d := approvalWorkflow(project.ID, "cancelled")
	submitted, err := backend.Submit(ctx, d, "approval-cancelled")
	if err != nil {
		t.Fatal(err)
	}
	awaitApproval(t, backend.Namespace(), submitted.RunID)
	if _, err = backend.CancelRun(ctx, submitted.RunID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		out, err := backend.GetRun(ctx, submitted.RunID)
		if err == nil && out.Status == "cancelled" && out.StopConfirmed {
			if model.count("cancelled") != 0 {
				t.Fatal("取消后执行模型")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("待审批运行无法取消")
}
