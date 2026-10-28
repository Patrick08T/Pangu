package api

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"testing"
)

type feedbackBackend struct {
	authorizedBackend
	project, owner, status string
}

func (b *feedbackBackend) GetRun(_ context.Context, id string) (*scheduler.RunDetails, error) {
	return &scheduler.RunDetails{RunSummary: scheduler.RunSummary{ID: id, ProjectID: b.project, CreatedBy: b.owner, Status: b.status}, StopConfirmed: true}, nil
}

func (b *feedbackBackend) Submit(ctx context.Context, d workflow.Definition, key string) (*submission.Record, error) {
	record, err := submission.Prepare(ctx, b.namespace, "submit", key, submission.Snapshot{Workflow: d})
	if err != nil {
		return nil, err
	}
	return submission.Dispatch(ctx, b.namespace, record.ID, func(context.Context, *submission.Record) (string, error) { return uuid.NewString(), nil })
}

func feedbackAPI(t *testing.T) (http.Handler, string, string, string, string) {
	t.Helper()
	_, ns, project, _, token := authorizationAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var user string
	if err = conn.QueryRow(ctx, "SELECT user_id::text FROM pangu.memberships WHERE project_id=$1", project).Scan(&user); err != nil {
		t.Fatal(err)
	}
	record, err := submission.Prepare(identity.WithActor(ctx, identity.Actor{ID: user}), ns, "submit", "feedback", submission.Snapshot{Workflow: workflow.Definition{ProjectID: project}})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, ns, record.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	h := New(&feedbackBackend{authorizedBackend: authorizedBackend{namespace: ns}, project: project, owner: user, status: "succeeded"}, identity.Service{Namespace: ns})
	return h, ns, project, run, token
}

func TestFeedbackRequiresOwnerAuthorityAndCurrentArtifact(t *testing.T) {
	h, _, project, run, token := feedbackAPI(t)
	path := "/runs/" + run + "/feedback"
	w := callAuthorized(h, "GET", path, "", token)
	var current struct {
		SubjectHash string `json:"subject_hash"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &current); err != nil || w.Code != 200 || len(current.SubjectHash) != 64 {
		t.Fatalf("评价对象缺失: %d %s", w.Code, w.Body.String())
	}
	body := `{"revision":0,"decision":"accepted","reason":"核对测试通过","subject_hash":"` + current.SubjectHash + `"}`
	if w = callAuthorized(h, "PUT", path, body, token); w.Code != 403 {
		t.Fatalf("查看者评价: %d %s", w.Code, w.Body.String())
	}
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w = callAuthorized(h, "PUT", path, strings.ReplaceAll(body, current.SubjectHash, strings.Repeat("b", 64)), token); w.Code != 409 {
		t.Fatalf("未核对当前产物: %d %s", w.Code, w.Body.String())
	}
	if w = callAuthorized(h, "PUT", path, body, token); w.Code != 200 {
		t.Fatalf("无法采纳: %d %s", w.Code, w.Body.String())
	}
	if w = callAuthorized(h, "PUT", path, body, token); w.Code != 200 {
		t.Fatalf("回执重试失败: %d %s", w.Code, w.Body.String())
	}
	if w = callAuthorized(h, "GET", "/projects/"+project+"/quality", "", token); w.Code != 200 || !strings.Contains(w.Body.String(), `"accepted_tasks":1`) {
		t.Fatalf("质量统计: %d %s", w.Code, w.Body.String())
	}
}

func TestEvaluationRunUsesFixedSampleAndVersion(t *testing.T) {
	h, ns, project, _, token := feedbackAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	body := `{"project_id":"` + project + `","name":"固定案例","input":"原始样本","expected":"满足验收","criteria":["输出完整"],"retain_input":true}`
	w := callAuthorized(h, "POST", "/evaluations/cases", body, token)
	var item struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &item); err != nil || w.Code != 201 {
		t.Fatalf("创建样本: %d %s", w.Code, w.Body.String())
	}
	object, err := catalog.Create(ctx, ns, catalog.NewObject{Kind: "agent", Name: "固定模型", Spec: catalog.Spec{Agent: &workflow.Node{ID: "one", Executor: "codex", Handoff: testHandoff(), Instruction: "测试"}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, ns, object.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	path := "/evaluations/cases/" + item.ID + "/runs"
	w = callAuthorized(h, "POST", path, `{"version_id":"`+strings.ToUpper(version.ID)+`"}`, token)
	if w.Code != 202 {
		t.Fatalf("启动评测: %d %s", w.Code, w.Body.String())
	}
	var receipt submission.Record
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	saved, err := submission.Get(ctx, ns, receipt.ID)
	if err != nil || saved.Snapshot.Workflow.Input != "原始样本" || saved.Snapshot.Workflow.VersionID != version.ID {
		t.Fatalf("输入或版本变化: %+v %v", saved, err)
	}
	w = callAuthorized(h, "POST", path, `{"version_id":"`+version.ID+`"}`, token)
	if w.Code != 202 || !strings.Contains(w.Body.String(), receipt.ID) {
		t.Fatalf("重试创建另一运行: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", path, "", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), receipt.ID) {
		t.Fatalf("缺少比较结果: %d %s", w.Code, w.Body.String())
	}
}
