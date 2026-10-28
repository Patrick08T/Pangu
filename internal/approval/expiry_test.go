package approval

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
	"time"
)

func TestApprovalDeadlineAfterAuthorizationLockWait(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	input := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "build", Executor: "codex", Instruction: "实施", Approval: &workflow.Approval{TimeoutSeconds: 600}}}
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Decide(ctx, actor, gate.ID, Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Reason: "通过"}); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = identity.Lock(ctx, tx, actor.Namespace); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- Consume(ctx, run, input, gate.ID) }()
	deadline := time.Now().Add(5 * time.Second)
	waiting := false
	for !waiting && time.Now().Before(deadline) {
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_locks a WHERE NOT a.granted AND pg_backend_pid()=ANY(pg_blocking_pids(a.pid)))").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if !waiting {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if !waiting {
		t.Fatal("消费事务未进入锁等待")
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.approvals SET expires_at=clock_timestamp() WHERE id=$1", gate.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; !errors.Is(err, ErrExpired) {
		t.Fatalf("锁等待越过绝对期限仍执行: %v", err)
	}
}
