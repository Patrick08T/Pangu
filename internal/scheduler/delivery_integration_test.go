package scheduler_test

import (
	"archive/zip"
	"bytes"
	"context"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/delivery"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
)

func TestHatchetDeliveryRequiresApprovalVerificationAndHumanAcceptance(t *testing.T) {
	backend, count := forkWorker(t)
	original := registerTestProject(t, backend)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	project, err := backend.RegisterProject(context.Background(), workspace.Project{Name: "交付链路", Source: original.Source, Branch: "main", WorkerID: original.WorkerID, Verification: []string{git, "diff", "--check"}})
	if err != nil {
		t.Fatal(err)
	}
	actor := approvalActor(t, backend.Namespace())
	ctx := identity.WithActor(context.Background(), actor)
	handler := api.New(backend, integrationAccess{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(identity.WithActor(r.Context(), actor)))
	})
	source, err := backend.Submit(ctx, workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "plan", Executor: "codex", Handoff: testHandoff(), Instruction: "准备任务"}}}, "source")
	if err != nil {
		t.Fatal(err)
	}
	_ = waitRun(t, h, source.RunID)
	report, input := deliveryDesign(t, backend, project.ID, source.RunID)
	var draft delivery.Draft
	request(t, h, "POST", "/runs/"+source.RunID+"/reports/"+report+"/deliveries", input, 201, &draft)
	var accepted submission.Record
	request(t, h, "POST", "/deliveries/"+draft.ID+"/run", map[string]string{"subject_hash": draft.SubjectHash}, 202, &accepted)
	gate := awaitApproval(t, backend.Namespace(), accepted.RunID)
	if count("implementation") != 0 {
		t.Fatal("未批准即开发")
	}
	if _, err = approval.Decide(ctx, actor, gate.ID, approval.Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Reason: "确认任务与验收范围"}); err != nil {
		t.Fatal(err)
	}
	if err = backend.ReconcileApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	out := waitRun(t, h, accepted.RunID)
	if len(out.Nodes) != 2 || count("implementation") != 1 || count("review") != 1 {
		t.Fatalf("交付执行重复或缺失 %+v", out)
	}
	assertDeliveryAcceptance(t, h, backend.Namespace(), draft.ID, accepted.RunID)
}

func deliveryDesign(t *testing.T, b scheduler.Backend, project, run string) (string, delivery.CreateInput) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	report := uuid.NewString()
	metadata := `{"mode":"design","tasks":[{"id":"api","title":"接口","scope":"一个文件","depends_on":[],"deliverable":"代码","acceptance":"验证通过"}],"claims":[],"sources":[]}`
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence,metadata) VALUES($1,$2,$3,'设计',$4,'{}','[]',$5)`, report, run, uuid.NewString(), []byte("模拟设计正文"), metadata); err != nil {
		t.Fatal(err)
	}
	detail, err := research.ReadDetail(ctx, b.Namespace(), run, report)
	if err != nil {
		t.Fatal(err)
	}
	input := delivery.CreateInput{TaskIDs: []string{"api"}, SubjectHash: detail.SubjectHash}
	for i := range 2 {
		object, err := catalog.Create(ctx, b.Namespace(), catalog.NewObject{Kind: "agent", Name: uuid.NewString(), Spec: catalog.Spec{Agent: &workflow.Node{ID: "code", Executor: "codex", Handoff: testHandoff(), Instruction: "执行确认任务", ReadOnly: i == 1}}})
		if err != nil {
			t.Fatal(err)
		}
		version, err := catalog.Publish(ctx, b.Namespace(), object.ID, object.Revision)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			input.ImplementationVersion = version.ID
		} else {
			input.ReviewVersion = version.ID
		}
	}
	return report, input
}

func assertDeliveryAcceptance(t *testing.T, h http.Handler, ns, draft, run string) {
	t.Helper()
	ctx := context.Background()
	path := "/deliveries/" + draft + "/artifacts.zip"
	request(t, h, "GET", path, nil, 409, nil)
	var state struct {
		SubjectHash string `json:"subject_hash"`
	}
	request(t, h, "GET", "/runs/"+run+"/feedback", nil, 200, &state)
	request(t, h, "PUT", "/runs/"+run+"/feedback", evaluation.FeedbackInput{SubjectHash: state.SubjectHash, Decision: "accepted", Reason: "核验差异和测试证据"}, 200, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != 200 {
		t.Fatalf("验收后无法交付 %d %s", w.Code, w.Body.String())
	}
	if _, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len())); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.feedback SET source='unknown' WHERE run_id=$1", run); err != nil {
		t.Fatal(err)
	}
	request(t, h, "GET", path, nil, 409, nil)
	if _, err = conn.Exec(ctx, "UPDATE pangu.feedback SET source='feishu_session' WHERE run_id=$1", run); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.workspace_checkpoints SET evidence='[]' WHERE run_id=$1", run); err != nil {
		t.Fatal(err)
	}
	verified, err := delivery.Verified(ctx, ns, draft)
	if err != nil || verified {
		t.Fatalf("缺验证仍可交付 %v %v", verified, err)
	}
	request(t, h, "GET", path, nil, 409, nil)
}
