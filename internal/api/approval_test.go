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
	gate, err := approval.Request(ctx, ns, request, run, workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "build", Executor: "codex", Handoff: testHandoff(), Instruction: "实现", DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}, Parents: map[string]workflow.NodeOutput{"plan": {Text: `{"review":{"title":"范围","summary":"审核范围","items":[{"id":"scope","category":"范围","title":"文档","proposal":"仅文档","rationale":"验证","evidence_refs":[],"task_ids":[]}],"resolutions":[]}}`}}})
	if err != nil {
		t.Fatal(err)
	}
	w := callAuthorized(h, "GET", "/runs/"+run+"/approvals", "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), gate.SubjectHash) {
		t.Fatalf("审批列表: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", "/approvals?project_id="+project, "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), gate.ID) || strings.Contains(w.Body.String(), "node_input") {
		t.Fatalf("专用审核列表: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", "/approvals/"+gate.ID, "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"review"`) {
		t.Fatalf("专用审核详情: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", "/approvals?project_id=00000000-0000-4000-8000-000000000099", "", token)
	if w.Code != 403 {
		t.Fatalf("列表未拒绝跨项目查询: %d", w.Code)
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
