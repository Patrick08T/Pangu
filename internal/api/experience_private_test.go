//go:build integration

package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"strings"
	"testing"
)

type privateRunBackend struct {
	authorizedBackend
	project, purpose string
}

func (b *privateRunBackend) GetRun(_ context.Context, id string) (*scheduler.RunDetails, error) {
	return &scheduler.RunDetails{RunSummary: scheduler.RunSummary{ID: id, ProjectID: b.project, Purpose: b.purpose, Title: "hidden-case-input"}}, nil
}
func (b *privateRunBackend) GetRequest(_ context.Context, id string) (*submission.Record, error) {
	return &submission.Record{ID: id, ProjectID: b.project, Snapshot: submission.Snapshot{Purpose: b.purpose}}, nil
}
func (b *privateRunBackend) ListRuns(_ context.Context, _ scheduler.RunQuery) (*scheduler.RunList, error) {
	return &scheduler.RunList{Items: []scheduler.RunSummary{{ID: uuid.NewString(), ProjectID: b.project, Purpose: b.purpose, Title: "hidden-case-input"}}}, nil
}
func TestExperienceInternalRunsCannotUseBusinessReadSurfaces(t *testing.T) {
	f := experienceAPI(t)
	b := &privateRunBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, project: f.project}
	h := New(b, identity.Service{Namespace: f.namespace})
	for _, purpose := range []string{"evaluation", "learning", "candidate", "unrecognized-internal"} {
		b.purpose = purpose
		for _, tail := range []string{"", "/usage", "/trace", "/artifacts", "/inputs", "/events", "/reports", "/activity"} {
			w := callAuthorized(h, "GET", "/runs/"+uuid.NewString()+tail, "", f.session)
			if w.Code != 404 || strings.Contains(w.Body.String(), "hidden-case-input") {
				t.Fatalf("%s %s: %d %s", purpose, tail, w.Code, w.Body.String())
			}
		}
		w := callAuthorized(h, "GET", "/requests/"+uuid.NewString(), "", f.session)
		if w.Code != 404 {
			t.Fatalf("private request: %d %s", w.Code, w.Body.String())
		}
		w = callAuthorized(h, "GET", "/runs?project_id="+f.project, "", f.session)
		if w.Code != 200 || strings.Contains(w.Body.String(), "hidden-case-input") {
			t.Fatalf("private list: %d %s", w.Code, w.Body.String())
		}
	}
	b.purpose = "business"
	w := callAuthorized(h, "GET", "/runs/"+uuid.NewString(), "", f.session)
	if w.Code != 200 {
		t.Fatalf("business blocked: %d %s", w.Code, w.Body.String())
	}
}
