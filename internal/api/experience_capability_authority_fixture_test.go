//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
)

type capabilityRuntimeBackend struct{ authorizedBackend }

func (b *capabilityRuntimeBackend) EvolutionService() evolution.Service {
	return (scheduler.Backend{}).EvolutionService()
}

func (b *capabilityRuntimeBackend) SubmitEntry(ctx context.Context, project, entry, input, key string) (*submission.Record, error) {
	return (scheduler.Backend{}).SubmitEntry(ctx, project, entry, input, key)
}

type capabilityResponse struct {
	Permissions map[string]bool `json:"permissions"`
	Features    map[string]bool `json:"features"`
}

func capabilityHandler(f experienceFixture, ready bool) http.Handler {
	if !ready {
		return f.handler
	}
	return New(&capabilityRuntimeBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}}, identity.Service{Namespace: f.namespace})
}

func capabilityExec(t *testing.T, query string, args ...any) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func capabilityRead(t *testing.T, h http.Handler, f experienceFixture) capabilityResponse {
	t.Helper()
	w := callAuthorized(h, "GET", "/projects/"+f.project+"/experience-capabilities", "", f.session)
	if w.Code != 200 {
		t.Fatalf("capabilities status=%d body=%s", w.Code, w.Body.String())
	}
	var result capabilityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func capabilityPermissions(t *testing.T, got capabilityResponse, manage, submit, rollback bool) {
	t.Helper()
	for _, name := range []string{"manage_policy", "manage_sources", "revoke", "delete"} {
		if value, ok := got.Permissions[name]; !ok || value != manage {
			t.Errorf("%s=%v present=%v want=%v", name, value, ok, manage)
		}
	}
	for name, want := range map[string]bool{"submit_entry": submit, "rollback": rollback} {
		if value, ok := got.Permissions[name]; !ok || value != want {
			t.Errorf("%s=%v present=%v want=%v", name, value, ok, want)
		}
	}
}

func capabilityBearer(h http.Handler, path, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
