//go:build integration

package api

import (
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestExperienceReadBoundariesAndPagination(t *testing.T) {
	f := experienceAPI(t)
	for _, suffix := range []string{"/experiences", "/experience-capabilities"} {
		w := callAuthorized(f.handler, "GET", "/projects/"+f.project+suffix, "", f.session)
		if w.Code != 200 {
			t.Fatalf("read %s: %d %s", suffix, w.Code, w.Body.String())
		}
		w = callAuthorized(f.handler, "GET", "/projects/"+f.other+suffix, "", f.session)
		if w.Code != 403 {
			t.Fatalf("cross-project %s: %d", suffix, w.Code)
		}
	}
	for _, query := range []string{"?limit=101", "?limit=0", "?cursor=not-an-id"} {
		w := callAuthorized(f.handler, "GET", "/projects/"+f.project+"/experiences"+query, "", f.session)
		if w.Code != 400 {
			t.Fatalf("invalid page %s: %d %s", query, w.Code, w.Body.String())
		}
	}
	w := callAuthorized(f.handler, "GET", "/projects/"+f.project+"/experiences", "", f.session)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("empty page must be explicit", w.Body.String())
	}
}

func TestExperienceRefusesClientReceiptsAndHiddenEvidence(t *testing.T) {
	f := experienceAPI(t)
	base := "/projects/" + f.project + "/experiences/" + uuid.NewString()
	for _, suffix := range []string{"/activate", "/delivery"} {
		w := callAuthorized(f.handler, "POST", base+suffix, `{"passed":true,"receipt_id":"pretend"}`, f.session)
		if w.Code != 400 {
			t.Fatalf("forged %s: %d %s", suffix, w.Code, w.Body.String())
		}
	}
	for _, suffix := range []string{"/revoke", ""} {
		method := "POST"
		if suffix == "" {
			method = "DELETE"
		}
		w := callAuthorized(f.handler, method, base+suffix, `{"reason":"test","project_id":"`+f.other+`"}`, f.session)
		if w.Code != 400 {
			t.Fatalf("forged scope: %d %s", w.Code, w.Body.String())
		}
	}
	f.role(t, "viewer")
	w := callAuthorized(f.handler, "POST", base+"/revoke", `{"reason":"test"}`, f.session)
	if w.Code != 403 {
		t.Fatalf("viewer governance: %d %s", w.Code, w.Body.String())
	}
}
