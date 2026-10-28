package approval

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"os"
	"strings"
	"testing"
)

func approvalFixture(t *testing.T) (identity.Actor, string, string, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要测试数据库")
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
	actor := identity.Actor{ID: uuid.NewString(), Namespace: uuid.NewString()}
	project, run := uuid.NewString(), uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,$2,'审批测试','unused','main','test')`, project, actor.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, actor.ID, actor.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	d := workflow.Definition{ProjectID: project, Input: "方案", Nodes: []workflow.Node{{ID: "build", Executor: "codex", Instruction: "实施", Approval: &workflow.Approval{TimeoutSeconds: 600}}}}
	request, err := submission.Prepare(identity.WithActor(ctx, actor), actor.Namespace, "submit", "test", submission.Snapshot{Workflow: d})
	if err != nil {
		t.Fatal(err)
	}
	_, err = submission.Dispatch(ctx, actor.Namespace, request.ID, func(context.Context, *submission.Record) (string, error) { return run, nil })
	if err != nil {
		t.Fatal(err)
	}
	return actor, project, run, request.ID
}

func TestApprovalBindsSubjectAndConsumesOnce(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	input := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "build", Executor: "codex", Instruction: "实施", Approval: &workflow.Approval{TimeoutSeconds: 600}}, Input: "方案"}
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("未批准执行: %v", err)
	}
	decision := Decision{SubjectHash: strings.Repeat("a", 64), Decision: "approved", Reason: "方案已核对"}
	if _, err = Decide(ctx, actor, gate.ID, decision); !errors.Is(err, ErrConflict) {
		t.Fatalf("接受错误摘要: %v", err)
	}
	decision.SubjectHash = gate.SubjectHash
	if _, err = Decide(ctx, actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
	if _, err = Decide(ctx, actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); err != nil {
		t.Fatalf("重启重试不能恢复: %v", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.approvals SET head_commit='changed' WHERE id=$1", gate.ID); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("新尝试越过已批准检查点: %v", err)
	}
	decision.Decision = "revoked"
	if _, err = Decide(ctx, actor, gate.ID, decision); !errors.Is(err, ErrConflict) {
		t.Fatalf("已执行批准被撤回: %v", err)
	}
	input.Input = "另一方案"
	if _, err = Request(ctx, actor.Namespace, request, run, input); !errors.Is(err, ErrConflict) {
		t.Fatalf("内容变化复用批准: %v", err)
	}
}

func TestApprovalExpiryAndRevocationBlockConsumption(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	input := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "build", Executor: "codex", Instruction: "实施", Approval: &workflow.Approval{TimeoutSeconds: 600}}}
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	decision := Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Reason: "核验通过"}
	if _, err = Decide(ctx, actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.users SET disabled=true WHERE id=$1", actor.ID); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("撤权仍执行: %v", err)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.users SET disabled=false WHERE id=$1", actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.approvals SET expires_at=now()-interval '1 second' WHERE id=$1", gate.ID); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); !errors.Is(err, ErrExpired) {
		t.Fatalf("过期仍执行: %v", err)
	}
	if _, err = Decide(ctx, actor, gate.ID, decision); !errors.Is(err, ErrExpired) {
		t.Fatalf("过期重放仍批准: %v", err)
	}
}
