//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

type entryHTTPBackend struct {
	authorizedBackend
	project, entry, input, key string
	empty                      bool
}

func (b *entryHTTPBackend) SubmitEntry(_ context.Context, project, entry, input, key string) (*submission.Record, error) {
	b.project, b.entry, b.input, b.key = project, entry, input, key
	if b.empty {
		return nil, nil
	}
	return &submission.Record{ID: uuid.NewString(), ProjectID: project, Status: "pending"}, nil
}

func TestExperienceEntryCapabilitiesAndPublicIdempotency(t *testing.T) {
	f := experienceAPI(t)
	b := &entryHTTPBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}}
	h := New(b, identity.Service{Namespace: f.namespace})
	for _, test := range []struct {
		handler http.Handler
		ready   bool
	}{{f.handler, false}, {h, true}} {
		w := callAuthorized(test.handler, "GET", "/projects/"+f.project+"/experience-capabilities", "", f.session)
		var caps struct {
			Features    map[string]bool `json:"features"`
			Permissions map[string]bool `json:"permissions"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &caps); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || caps.Features["entry_submission"] != test.ready || !caps.Permissions["submit_entry"] {
			t.Fatalf("entry capabilities: %d %s", w.Code, w.Body.String())
		}
	}
	for _, key := range []string{"", "eval:forged"} {
		wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { r.Header.Set("Idempotency-Key", key); h.ServeHTTP(w, r) })
		w := callAuthorized(wrapped, "POST", "/projects/"+f.project+"/entries/default/runs", `{"input":"task"}`, f.session)
		if w.Code != 400 {
			t.Fatalf("private/missing key %q: %d %s", key, w.Code, w.Body.String())
		}
	}
	f.role(t, "viewer")
	w := callAuthorized(h, "GET", "/projects/"+f.project+"/experience-capabilities", "", f.session)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"submit_entry":false`) {
		t.Fatalf("read-only entry permission: %d %s", w.Code, w.Body.String())
	}
}

func TestExperienceEntrySubmissionBoundaries(t *testing.T) {
	f := experienceAPI(t)
	path := "/projects/" + f.project + "/entries/default/runs"
	w := callAuthorized(f.handler, "POST", path, `{"input":"business task"}`, f.session)
	if w.Code != 503 {
		t.Fatalf("unwired entry: %d %s", w.Code, w.Body.String())
	}
	for _, field := range []string{"version_id", "entry_revision", "entry_key", "purpose", "score"} {
		w = callAuthorized(f.handler, "POST", path, `{"input":"business task","`+field+`":"forged"}`, f.session)
		if w.Code != 400 {
			t.Fatalf("forged %s: %d %s", field, w.Code, w.Body.String())
		}
	}
	b := &entryHTTPBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}}
	h := New(b, identity.Service{Namespace: f.namespace})
	w = callAuthorized(h, "POST", path, `{"input":"business task"}`, f.session)
	if w.Code != 202 || b.project != f.project || b.entry != "default" || b.input != "business task" || b.key != "authorization-test" || !strings.Contains(w.Body.String(), `"status":"pending"`) {
		t.Fatalf("entry forwarding: %d %s %+v", w.Code, w.Body.String(), b)
	}
	w = callAuthorized(h, "POST", strings.Replace(path, f.project, f.other, 1), `{"input":"business task"}`, f.session)
	if w.Code != 403 {
		t.Fatalf("cross project: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "POST", strings.Replace(path, "default", "cron:other", 1), `{"input":"business task"}`, f.session)
	if w.Code != 400 {
		t.Fatalf("cross entry kind: %d %s", w.Code, w.Body.String())
	}
	b.empty = true
	w = callAuthorized(h, "POST", path, `{"input":"business task"}`, f.session)
	if w.Code != 503 {
		t.Fatalf("empty accepted receipt: %d %s", w.Code, w.Body.String())
	}
}
