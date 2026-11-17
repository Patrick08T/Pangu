package approval

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
)

func TestApprovalWaitPersistsUntilDecision(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	input := structuredInput(project, "方案")
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.approvals SET created_at=now()-interval '90 days' WHERE id=$1", gate.ID); err != nil {
		t.Fatal(err)
	}
	current, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil || current.ID != gate.ID || current.Status != "waiting" {
		t.Fatalf("长时间等待或恢复后审批丢失: %+v %v", current, err)
	}
	raw, _ := json.Marshal(current)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	if _, exists := fields["expires_at"]; exists {
		t.Fatal("审批仍暴露过期时间")
	}
	if _, err = Decide(ctx, actor, gate.ID, Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: acceptedItems()}); err != nil {
		t.Fatal(err)
	}
	ids, err := PendingEvents(ctx, actor.Namespace)
	if err != nil || len(ids) != 1 || ids[0] != gate.ID {
		t.Fatalf("批准事件丢失: %v %v", ids, err)
	}
	if err = Consume(ctx, run, input, gate.ID); err != nil {
		t.Fatal(err)
	}
}
