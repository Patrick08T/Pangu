package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"strings"
	"testing"
)

func TestApprovalRoutesRequireOwnerAndCurrentSubject(t *testing.T) {
	h, ns, project, run, token := feedbackAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var request string
	if err = conn.QueryRow(ctx, "SELECT id::text FROM pangu.submissions WHERE run_id=$1", run).Scan(&request); err != nil {
		t.Fatal(err)
	}
	gate, err := approval.Request(ctx, ns, request, run, workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "build", Executor: "codex", Handoff: testHandoff(), Instruction: "实现", Approval: &workflow.Approval{TimeoutSeconds: 600}}})
	if err != nil {
		t.Fatal(err)
	}
	w := callAuthorized(h, "GET", "/runs/"+run+"/approvals", "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), gate.SubjectHash) {
		t.Fatalf("审批列表: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "PUT", "/approvals/"+gate.ID, `{"subject_hash":"`+gate.SubjectHash+`","decision":"approved","reason":"核验通过"}`, token)
	if w.Code != 403 {
		t.Fatalf("查看者可批准: %d %s", w.Code, w.Body.String())
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	w = callAuthorized(h, "PUT", "/approvals/"+gate.ID, `{"subject_hash":"`+gate.SubjectHash+`","decision":"approved","reason":"核验通过"}`, token)
	if w.Code != 409 {
		t.Fatalf("终态运行仍可批准: %d %s", w.Code, w.Body.String())
	}
}
