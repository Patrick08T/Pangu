package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func (b *fakeBackend) CheckProject(context.Context, workspace.Project) (*workflow.Preflight, error) {
	b.readCall()
	return &workflow.Preflight{Ready: true, Checks: []workflow.Check{}}, b.err
}
func (b *fakeBackend) DeliverPerforce(context.Context, string) (*workspace.P4Delivery, error) {
	b.mutationCall()
	return &workspace.P4Delivery{Status: "shelved", Changelist: 1}, b.err
}

func (b *fakeBackend) RegisterProject(_ context.Context, value workspace.Project) (*workspace.Project, error) {
	b.mutationCall()
	return &value, b.err
}

func (b *fakeBackend) Projects(context.Context) ([]workspace.Project, error) {
	b.readCall()
	return []workspace.Project{}, b.err
}

func TestRejectClientChosenProjectIDAndRelativeDirectory(t *testing.T) {
	for _, input := range []string{`{"name":"项目","source":"relative","branch":"main","worker_id":"local"}`,
		`{"id":"chosen","name":"项目","source":"C:/repo","branch":"main","worker_id":"local"}`} {
		backend := &fakeBackend{}
		request := httptest.NewRequest("POST", "/projects", strings.NewReader(input))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		testAPI(backend).ServeHTTP(response, request)
		if response.Code != 400 || backend.calls != 0 {
			t.Fatalf("接受非法项目: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestPerforceDeliveryAcknowledgesBackgroundDispatch(t *testing.T) {
	b := &fakeBackend{}
	r := httptest.NewRequest("POST", "/runs/"+testID+"/perforce", nil)
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, r)
	if w.Code != 202 || b.mutations != 1 || !strings.Contains(w.Body.String(), `"status":"shelved"`) {
		t.Fatalf("交付必须异步受理: %d %s", w.Code, w.Body.String())
	}
}
