package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/Patrick08T/Pangu/web"
)

const validRun = `{"project_id":"eb1d38c7-d454-4bc7-a337-827b76a5e6c8","input":"主题","nodes":[{"id":"writer","executor":"eino","model":"test","instruction":"撰写"}]}`
const testID = "eb1d38c7-d454-4bc7-a337-827b76a5e6c8"

type fakeBackend struct {
	calls      int
	definition workflow.Definition
	cron       workflow.Cron
	err        error
}

func (b *fakeBackend) Check(context.Context) error { return b.err }
func (b *fakeBackend) Retention(context.Context, bool) ([]workspace.RetentionItem, error) {
	b.calls++
	return []workspace.RetentionItem{}, b.err
}

func (b *fakeBackend) Namespace() string { return "api-test" }

func (b *fakeBackend) Submit(_ context.Context, d workflow.Definition, _ string) (*submission.Record, error) {
	b.calls++
	b.definition = d
	return &submission.Record{ID: testID, RunID: testID, Status: "accepted"}, b.err
}
func (b *fakeBackend) GetRequest(context.Context, string) (*submission.Record, error) {
	b.calls++
	return &submission.Record{ID: testID}, b.err
}

func (b *fakeBackend) Rerun(context.Context, string, string) (*submission.Record, error) {
	b.calls++
	return &submission.Record{ID: testID}, b.err
}
func (b *fakeBackend) GetRun(context.Context, string) (*scheduler.RunDetails, error) {
	b.calls++
	return &scheduler.RunDetails{}, b.err
}
func (b *fakeBackend) Events(context.Context, string, string, int, int) (*scheduler.EventPage, error) {
	b.calls++
	return &scheduler.EventPage{Items: []scheduler.RunEvent{}}, b.err
}
func (b *fakeBackend) CreateCron(_ context.Context, c workflow.Cron) (*schedule.Plan, error) {
	b.calls++
	b.cron = c
	return &schedule.Plan{}, b.err
}

func (b *fakeBackend) UpdateCron(_ context.Context, _ string, _ int, c workflow.Cron, _ bool) (*schedule.Plan, error) {
	b.calls++
	b.cron = c
	return &schedule.Plan{}, b.err
}
func (b *fakeBackend) ListCrons(context.Context, int64) ([]schedule.Plan, error) {
	b.calls++
	return []schedule.Plan{}, b.err
}
func (b *fakeBackend) DeleteCron(context.Context, string) error { b.calls++; return b.err }
func (b *fakeBackend) GetCron(context.Context, string) (*schedule.Plan, error) {
	return &schedule.Plan{Workflow: workflow.Definition{ProjectID: testID}}, b.err
}

func (b *fakeBackend) CronFires(context.Context, string, int64) ([]schedule.Fire, error) {
	b.calls++
	return []schedule.Fire{}, b.err
}

func TestSubmit(t *testing.T) {
	b := &fakeBackend{}
	r := httptest.NewRequest("POST", "/runs", strings.NewReader(validRun))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "test-submit")
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, r)
	if w.Code != 202 || !strings.Contains(w.Body.String(), testID) || b.definition.Nodes[0].ID != "writer" {
		t.Fatalf("code=%d body=%s backend=%+v", w.Code, w.Body.String(), b)
	}
}

func TestSubmissionLocationUsesPublicAPIPrefix(t *testing.T) {
	h := web.Handler(testAPI(&fakeBackend{}))
	for _, path := range []string{"/api/runs", "/api/runs/" + testID + "/rerun"} {
		request := httptest.NewRequest("POST", path, strings.NewReader(validRun))
		if strings.HasSuffix(path, "rerun") {
			request = httptest.NewRequest("POST", path, nil)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "receipt-location")
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		if response.Code != 202 {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		receipt := httptest.NewRecorder()
		h.ServeHTTP(receipt, httptest.NewRequest("GET", response.Header().Get("Location"), nil))
		if receipt.Code != 200 {
			t.Fatalf("回执路径无法查询: %s %d", response.Header().Get("Location"), receipt.Code)
		}
	}
}

func TestRejectInvalidRequests(t *testing.T) {
	cases := []struct {
		path, body, contentType string
		status                  int
	}{
		{"/runs", `{`, "application/json", 400},
		{"/runs", validRun + `{}`, "application/json", 400},
		{"/runs", `{"nodes":[],"typo":true}`, "application/json", 400},
		{"/runs", `{"nodes":[]}`, "application/json", 400},
		{"/runs", `{"nodes":[{"id":"coder","executor":"codex","instruction":"任务","workspace":"/tmp"}]}`, "application/json", 400},
		{"/runs", validRun, "text/plain", 415},
		{"/runs", strings.Repeat(" ", 256*1024) + validRun, "application/json", 413},
		{"/crons", `{"name":"test","expression":"bad","workflow":` + validRun + `}`, "application/json", 400},
	}
	for _, tc := range cases {
		t.Run(tc.path+tc.body[:1]+tc.contentType, func(t *testing.T) {
			b := &fakeBackend{}
			r := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			testAPI(b).ServeHTTP(w, r)
			if w.Code != tc.status || b.calls != 0 {
				t.Fatalf("status=%d body=%s calls=%d", w.Code, w.Body.String(), b.calls)
			}
		})
	}
}

func TestCronAndQueryRoutes(t *testing.T) {
	cases := []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/crons", `{"name":"daily","expression":"0 2 * * *","workflow":` + validRun + `}`, 201},
		{"GET", "/crons?offset=0", "", 200},
		{"DELETE", "/crons/" + testID, "", 204},
		{"GET", "/runs/" + testID, "", 200},
	}
	for _, tc := range cases {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			b := &fakeBackend{}
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			testAPI(b).ServeHTTP(w, r)
			if w.Code != tc.status || b.calls != 1 {
				t.Fatalf("code=%d calls=%d body=%s", w.Code, b.calls, w.Body.String())
			}
			if tc.method == "POST" && b.cron.Workflow.Nodes[0].ID != "writer" {
				t.Fatal("cron lost definition")
			}
		})
	}
}

func TestRejectInvalidIdentifiers(t *testing.T) {
	for _, path := range []string{"/runs/not-a-uuid", "/crons?offset=-1"} {
		b := &fakeBackend{}
		w := httptest.NewRecorder()
		testAPI(b).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 400 || b.calls != 0 {
			t.Fatalf("code=%d calls=%d", w.Code, b.calls)
		}
	}
}

func TestBackendErrorDoesNotLeakDetails(t *testing.T) {
	b := &fakeBackend{err: errors.New("secret credential")}
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, httptest.NewRequest("GET", "/runs/"+testID, nil))
	if w.Code != 502 || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
