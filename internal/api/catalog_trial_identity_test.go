package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"net/http"
	"strings"
	"testing"
)

type revokedTrialBackend struct {
	authorizedBackend
	revoke func()
}

func (b *revokedTrialBackend) Preflight(_ context.Context, _ workflow.Definition) (*workflow.Preflight, error) {
	b.revoke()
	return &workflow.Preflight{Ready: true}, nil
}

func TestAgentTrialRechecksPermissionAfterPreflight(t *testing.T) {
	_, namespace, project, _, session := authorizationAPI(t)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	item, err := catalog.Create(context.Background(), namespace, catalog.NewObject{Kind: "agent", Name: "撤权测试", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Handoff: testHandoff(), Instruction: "检查"}}})
	if err != nil {
		t.Fatal(err)
	}
	b := &revokedTrialBackend{authorizedBackend: authorizedBackend{namespace: namespace}, revoke: func() {
		if _, err := conn.Exec(context.Background(), "UPDATE pangu.memberships SET role='viewer' WHERE project_id=$1", project); err != nil {
			t.Fatal(err)
		}
	}}
	response := callAuthorized(New(b, identity.Service{Namespace: namespace}), http.MethodPost, "/catalog/"+item.ID+"/trials", `{"project_id":"`+project+`","revision":1,"input":"检查"}`, session)
	if response.Code != 403 {
		t.Fatalf("撤权后仍接受试运行: %d %s", response.Code, response.Body.String())
	}
	rows, err := submission.Trials(context.Background(), namespace, item.ID, project, 0)
	if err != nil || len(rows) != 0 {
		t.Fatal("撤权后持久化了试运行", err)
	}
}

func TestAgentTrialAuthenticatedProjectAndRoleBoundary(t *testing.T) {
	h, namespace, project, other, session := authorizationAPI(t)
	for _, p := range []string{project, other} {
		item, err := catalog.Create(context.Background(), namespace, catalog.NewObject{Kind: "agent", Name: "权限测试", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Handoff: testHandoff(), Instruction: "检查"}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			method, path string
			code         int
		}{{"GET", "/trials", 200}, {"POST", "/trials", 403}, {"POST", "/draft/preflight", 403}} {
			expected := test.code
			if p == other {
				expected = 403
			}
			path := "/catalog/" + item.ID + test.path
			if test.method == "GET" {
				path += "?project_id=" + p
			}
			response := callAuthorized(h, test.method, path, `{"project_id":"`+p+`","revision":1,"input":"检查"}`, session)
			if response.Code != expected {
				t.Fatalf("权限不符 %s %s: %d %s", test.method, test.path, response.Code, response.Body.String())
			}
		}
	}
}

func TestSharedAgentTrialHistoryIsSeparatedByExecutionProject(t *testing.T) {
	h, ns, project, other, session := authorizationAPI(t)
	ctx := context.Background()
	item, err := catalog.Create(ctx, ns, catalog.NewObject{Kind: "agent", Name: "共享试运行", Spec: catalog.Spec{Agent: &workflow.Node{ID: "check", Executor: "codex", Instruction: "检查", Handoff: testHandoff()}}})
	if err != nil {
		t.Fatal(err)
	}
	var own, foreign string
	for _, p := range []string{project, other} {
		d, err := catalog.CompileDraft(ctx, ns, item.ID, item.Revision, p, "项目输入")
		if err != nil {
			t.Fatal(err)
		}
		record, err := submission.PrepareTrial(ctx, ns, item.ID, item.Revision, *d, trialKey())
		if err != nil {
			t.Fatal(err)
		}
		if p == project {
			own = record.ID
		} else {
			foreign = record.ID
		}
	}
	w := callAuthorized(h, "GET", "/catalog/"+item.ID+"/trials?project_id="+project, "", session)
	if w.Code != 200 || !strings.Contains(w.Body.String(), own) || strings.Contains(w.Body.String(), foreign) {
		t.Fatalf("试运行历史跨项目泄漏: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "GET", "/catalog/"+item.ID+"/trials?project_id="+other, "", session)
	if w.Code != 403 {
		t.Fatalf("越权读取其他项目: %d", w.Code)
	}
}
