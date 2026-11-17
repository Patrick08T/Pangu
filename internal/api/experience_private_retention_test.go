//go:build integration

package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func TestExperiencePrivateRetentionUnknownRunSurfacesFailClosed(t *testing.T) {
	f := experienceAPI(t)
	run := seedRetentionRun(t, f, "evaluation", true, "")
	b := &retentionHTTPBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, runs: []string{run.id}}
	h := New(b, identity.Service{Namespace: f.namespace})
	for _, tail := range []string{"", "/usage", "/trace", "/artifacts", "/inputs", "/events", "/reports", "/activity", "/approvals", "/experience", "/rebuild-intents"} {
		t.Run(tail, func(t *testing.T) {
			w := callAuthorized(h, "GET", "/runs/"+run.id+tail+"?purpose=business", "", f.session)
			if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "restricted-metadata") {
				t.Fatalf("unknown purged run %s: %d %s", tail, w.Code, w.Body.String())
			}
		})
	}
	w := callAuthorized(h, "GET", "/approvals/"+run.gate, "", f.session)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown purged approval: %d %s", w.Code, w.Body.String())
	}
}

func TestExperiencePrivateRetentionBindingAndErrorsRemainFailClosed(t *testing.T) {
	f := experienceAPI(t)
	run := seedRetentionRun(t, f, "", false, "")
	for _, tc := range []struct{ namespace, project, run string }{
		{f.namespace, f.other, run.id},
		{uuid.NewString(), f.project, run.id},
		{f.namespace, f.project, uuid.NewString()},
	} {
		h := handler{backend: &retentionHTTPBackend{authorizedBackend: authorizedBackend{namespace: tc.namespace}}}
		visible, err := h.businessRun(context.Background(), scheduler.RunSummary{ID: tc.run, ProjectID: tc.project})
		if err != nil || visible {
			t.Fatalf("unproven binding allowed or misclassified: %v %v", visible, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := handler{backend: &retentionHTTPBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}}}
	visible, err := h.businessRun(ctx, scheduler.RunSummary{ID: run.id, ProjectID: f.project})
	if visible || err == nil || errors.Is(err, submission.ErrNotFound) || errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("canceled lookup must propagate infrastructure error rather than deny or allow: %v %v", visible, err)
	}
}

func TestExperiencePrivateRetentionListsKeepOnlyProvenBusiness(t *testing.T) {
	f := experienceAPI(t)
	unknown := seedRetentionRun(t, f, "evaluation", true, "")
	legacy := seedRetentionRun(t, f, "", false, "")
	business := seedRetentionRun(t, f, "business", true, "business")
	private := seedRetentionRun(t, f, "evaluation", false, "")
	b := &retentionHTTPBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, runs: []string{unknown.id, legacy.id, business.id, private.id}}
	h := New(b, identity.Service{Namespace: f.namespace})
	for _, path := range []string{"/runs?project_id=" + f.project, "/approvals?project_id=" + f.project} {
		w := callAuthorized(h, "GET", path, "", f.session)
		if w.Code != http.StatusOK {
			t.Fatalf("list %s: %d %s", path, w.Code, w.Body.String())
		}
		for _, hidden := range []string{unknown.id, unknown.gate, private.id, private.gate} {
			if strings.Contains(w.Body.String(), hidden) {
				t.Errorf("%s leaked private/unknown identity %s: %s", path, hidden, w.Body.String())
			}
		}
		for _, visible := range []string{legacy.id, business.id} {
			if !strings.Contains(w.Body.String(), visible) {
				t.Errorf("%s omitted proven business %s: %s", path, visible, w.Body.String())
			}
		}
	}
	for _, visible := range []retentionRun{legacy, business} {
		w := callAuthorized(h, "GET", "/runs/"+visible.id, "", f.session)
		if w.Code != http.StatusOK {
			t.Errorf("proven business detail: %d %s", w.Code, w.Body.String())
		}
	}
}
