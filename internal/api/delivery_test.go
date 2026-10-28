package api

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/delivery"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"testing"
)

func deliveryAPI(t *testing.T) (http.Handler, string, string, string, string, delivery.CreateInput) {
	t.Helper()
	h, ns, project, run, token := feedbackAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','test','unused',true)`, run, project); err != nil {
		t.Fatal(err)
	}
	report := uuid.NewString()
	metadata := `{"mode":"design","tasks":[{"id":"api","title":"接口","scope":"增加接口","depends_on":[],"deliverable":"代码","acceptance":"测试通过"}],"claims":[],"sources":[]}`
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence,metadata) VALUES($1,$2,$3,'设计',$4,'{}','[]',$5)`, report, run, uuid.NewString(), []byte("设计正文"), metadata); err != nil {
		t.Fatal(err)
	}
	input := delivery.CreateInput{TaskIDs: []string{"api"}}
	for i := range 2 {
		object, err := catalog.Create(ctx, ns, catalog.NewObject{Kind: "agent", Name: uuid.NewString(), Spec: catalog.Spec{Agent: &workflow.Node{ID: "code", Executor: "codex", Handoff: testHandoff(), Instruction: "完成输入指定任务", ReadOnly: i == 1}}})
		if err != nil {
			t.Fatal(err)
		}
		version, err := catalog.Publish(ctx, ns, object.ID, object.Revision)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			input.ImplementationVersion = version.ID
		} else {
			input.ReviewVersion = version.ID
		}
	}
	detail, err := research.ReadDetail(ctx, ns, run, report)
	if err != nil {
		t.Fatal(err)
	}
	input.SubjectHash = detail.SubjectHash
	return h, ns, project, run, token, input
}

func TestDeliveryRequiresVerificationAndApprovalAndKeepsReceipt(t *testing.T) {
	h, ns, project, run, token, input := deliveryAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	reports, err := research.Details(ctx, ns, run)
	if err != nil {
		t.Fatal(err)
	}
	path := "/runs/" + run + "/reports/" + reports[0].ID + "/deliveries"
	raw, _ := json.Marshal(input)
	if w := callAuthorized(h, "POST", path, string(raw), token); w.Code != 403 {
		t.Fatalf("查看者建草案 %d %s", w.Code, w.Body.String())
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='manager' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w := callAuthorized(h, "POST", path, string(raw), token); w.Code != 409 {
		t.Fatalf("没有真实验证命令 %d %s", w.Code, w.Body.String())
	}
	if _, err = conn.Exec(ctx, `UPDATE pangu.projects SET verification='["C:\\Windows\\System32\\cmd.exe","/c","exit","0"]' WHERE id=$1`, project); err != nil {
		t.Fatal(err)
	}
	w := callAuthorized(h, "POST", path, string(raw), token)
	var draft delivery.Draft
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &draft) != nil {
		t.Fatalf("创建 %d %s", w.Code, w.Body.String())
	}
	if draft.Definition.Nodes[0].Approval == nil || !draft.Definition.Nodes[1].ReadOnly {
		t.Fatal("丢失人工审批/只读审查")
	}
	if w = callAuthorized(h, "POST", "/deliveries/"+draft.ID+"/run", `{"subject_hash":"changed"}`, token); w.Code != 409 {
		t.Fatalf("变更草案 %d", w.Code)
	}
	body := `{"subject_hash":"` + draft.SubjectHash + `"}`
	w = callAuthorized(h, "POST", "/deliveries/"+draft.ID+"/run", body, token)
	if w.Code != 202 {
		t.Fatalf("发起 %d %s", w.Code, w.Body.String())
	}
	first := w.Body.String()
	if _, err = conn.Exec(ctx, `UPDATE pangu.deliveries SET request_id=NULL,definition=NULL WHERE id=$1`, draft.ID); err != nil {
		t.Fatal(err)
	}
	w = callAuthorized(h, "POST", "/deliveries/"+draft.ID+"/run", body, token)
	if w.Code != 202 || strings.TrimSpace(first) != strings.TrimSpace(w.Body.String()) {
		t.Fatalf("过期/回执落库失败后重复提交 %d %s", w.Code, w.Body.String())
	}
}
