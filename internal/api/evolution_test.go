//go:build integration

package api

import (
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestEvolutionHTTPReadIsolationAndMissingExecution(t *testing.T) {
	f := experienceAPI(t)
	for _, suffix := range []string{"/evolution/intents", "/evolution/environments"} {
		w := callAuthorized(f.handler, "GET", "/projects/"+f.project+suffix, "", f.session)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
			t.Fatalf("read: %d %s", w.Code, w.Body.String())
		}
		w = callAuthorized(f.handler, "GET", "/projects/"+f.other+suffix, "", f.session)
		if w.Code != 403 {
			t.Fatalf("foreign read: %d %s", w.Code, w.Body.String())
		}
	}
	base := "/projects/" + f.project + "/evolution/intents/" + uuid.NewString() + "/rollback"
	w := callAuthorized(f.handler, "POST", base, `{"generation":1,"reason":"撤销试用"}`, f.session)
	if w.Code != 503 {
		t.Fatalf("missing trusted execution: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(f.handler, "POST", base, `{"generation":1,"reason":"撤销","passed":true}`, f.session)
	if w.Code != 400 {
		t.Fatalf("forged results: %d %s", w.Code, w.Body.String())
	}
	f.role(t, "viewer")
	w = callAuthorized(f.handler, "GET", "/projects/"+f.project+"/evolution/intents", "", f.session)
	if w.Code != 403 {
		t.Fatalf("non-manager: %d %s", w.Code, w.Body.String())
	}
}
