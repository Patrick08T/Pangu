//go:build integration

package api

import (
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestExperienceRebuildReadAndUnwiredMutation(t *testing.T) {
	f := experienceAPI(t)
	b := &privateRunBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, project: f.project, purpose: "business"}
	h := New(b, identity.Service{Namespace: f.namespace})
	path := "/runs/" + uuid.NewString() + "/rebuild-intents"
	w := callAuthorized(h, "GET", path, "", f.session)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("rebuild list: %d %s", w.Code, w.Body.String())
	}
	path += "/" + uuid.NewString()
	w = callAuthorized(h, "POST", path, `{"successor":"forged"}`, f.session)
	if w.Code != 400 {
		t.Fatalf("client successor: %d %s", w.Code, w.Body.String())
	}
	w = callAuthorized(h, "POST", path, `{}`, f.session)
	if w.Code != 503 {
		t.Fatalf("unwired rebuild: %d %s", w.Code, w.Body.String())
	}
	b.project = f.other
	w = callAuthorized(h, "GET", strings.TrimSuffix(path, "/"+path[strings.LastIndex(path, "/")+1:]), "", f.session)
	if w.Code != 403 {
		t.Fatalf("cross-project rebuild: %d %s", w.Code, w.Body.String())
	}
}
