package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workspace"
)

func (b *fakeBackend) RegisterProject(_ context.Context, value workspace.Project) (*workspace.Project, error) {
	b.calls++
	return &value, b.err
}

func (b *fakeBackend) Projects(context.Context) ([]workspace.Project, error) {
	b.calls++
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
