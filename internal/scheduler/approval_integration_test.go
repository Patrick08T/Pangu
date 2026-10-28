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
	"os"
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

func TestHatchetApprovalSurvivesWorkerRestart(t *testing.T) {
	if os.Getenv("PANGU_TEST_BINARY") == "" {
		t.Skip("需要构建后的 Worker")
	}
	model, client := integrationClient(t)
	backend := scheduler.NewBackend(client)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	process := startProcess(t)
	awaitWorker(t, backend)
	d := workflow.Definition{ProjectID: project.ID, Input: "批准后执行", Nodes: []workflow.Node{{ID: "build", Executor: "eino", Model: "approved", Handoff: testHandoff(), Instruction: "执行", Approval: &workflow.Approval{TimeoutSeconds: 120}}}}
	submitted, err := backend.Submit(ctx, d, "approval-restart")
	if err != nil || submitted.RunID == "" {
		t.Fatalf("提交: %+v %v", submitted, err)
	}
	gate := awaitApproval(t, backend.Namespace(), submitted.RunID)
	if model.count("approved") != 0 {
		t.Fatal("未批准即执行")
	}
	if err = process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = process.Wait()
	decision := approval.Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Reason: "已核对固定输入与产物"}
	if _, err = approval.Decide(ctx, actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	startProcess(t)
	out := waitRun(t, api.New(backend, integrationAccess{}), submitted.RunID)
	if out.Status != "succeeded" || model.count("approved") != 1 {
		t.Fatalf("恢复或重复执行: %s calls=%d error=%+v", out.Status, model.count("approved"), out.Error)
	}
	current, err := approval.Get(ctx, backend.Namespace(), gate.ID)
	if err != nil || current.Status != "consumed" {
		t.Fatalf("批准未绑定实际执行: %+v %v", current, err)
	}
}

func TestHatchetApprovalExpiryNeverCallsModel(t *testing.T) {
	model, backend, _ := integrationSetup(t)
	project := registerTestProject(t, backend)
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	d := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "build", Executor: "eino", Model: "expired", Handoff: testHandoff(), Instruction: "禁止过期执行", Approval: &workflow.Approval{TimeoutSeconds: 1}}}}
	submitted, err := backend.Submit(ctx, d, "approval-expiry")
	if err != nil || submitted.RunID == "" {
		t.Fatalf("提交: %+v %v", submitted, err)
	}
	out := waitFailedRun(t, backend, submitted.RunID)
	if out.Status != "failed" || model.count("expired") != 0 {
		t.Fatalf("过期执行: %s calls=%d", out.Status, model.count("expired"))
	}
}
