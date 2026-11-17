//go:build integration

package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExperiencePolicyRejectsUnsupportedEnabledMetrics(t *testing.T) {
	f := experienceAPI(t)
	for _, metric := range []string{"cost", "rework", "duration"} {
		w := callAuthorized(f.handler, "PUT", "/projects/"+f.project+"/experience-policy", `{"expected_revision":0,"policy":{"enabled":true,"evaluation":{"primary_metric":"`+metric+`"}}}`, f.session)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "不支持") {
			t.Fatalf("unsupported %s: %d %s", metric, w.Code, w.Body.String())
		}
	}
}

func TestExperiencePolicyDefaultsCASAndProjectIsolation(t *testing.T) {
	f := experienceAPI(t)
	path := "/projects/" + f.project + "/experience-policy"
	w := callAuthorized(f.handler, "GET", path, "", f.session)
	var current struct {
		Revision    int64 `json:"revision"`
		Enabled     bool  `json:"enabled"`
		ReadEnabled bool  `json:"read_enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &current); err != nil || w.Code != 200 || current.Revision != 0 || current.Enabled || current.ReadEnabled {
		t.Fatalf("default policy: %d %s", w.Code, w.Body.String())
	}
	body := `{"expected_revision":0,"policy":{"enabled":false,"read_enabled":true}}`
	w = callAuthorized(f.handler, "PUT", path, body, f.session)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"read_enabled":true`) || !strings.Contains(w.Body.String(), `"revision":1`) {
		t.Fatalf("independent switches: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(f.handler, "PUT", path, body, f.session)
	if w.Code != 409 {
		t.Fatalf("stale CAS: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(f.handler, "GET", "/projects/"+f.other+"/experience-policy", "", f.session)
	if w.Code != 403 {
		t.Fatalf("cross project: %d %s", w.Code, w.Body.String())
	}
}

func TestExperiencePolicyRejectsForgedAuthorityMissingCASAndIncompleteEnable(t *testing.T) {
	f := experienceAPI(t)
	path := "/projects/" + f.project + "/experience-policy"
	for _, body := range []string{
		`{"policy":{"enabled":false}}`,
		`{"expected_revision":0,"policy":{"project_id":"` + f.other + `"}}`,
		`{"expected_revision":0,"policy":{"actor_id":"` + f.actor + `"}}`,
		`{"expected_revision":0,"policy":{"revision":1}}`,
		`{"expected_revision":0,"policy":{"enabled":true,"read_enabled":true}}`,
		`{"expected_revision":0,"policy":{"passed":true}}`,
	} {
		w := callAuthorized(f.handler, "PUT", path, body, f.session)
		if w.Code != 400 {
			t.Errorf("untrusted or incomplete body %s: %d %s", body, w.Code, w.Body.String())
		}
	}
	f.role(t, "viewer")
	w := callAuthorized(f.handler, "PUT", path, `{"expected_revision":0,"policy":{"enabled":false}}`, f.session)
	if w.Code != 403 {
		t.Fatalf("viewer write: %d %s", w.Code, w.Body.String())
	}
}
