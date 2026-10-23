package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateDoesNotSubmit(t *testing.T) {
	b := &fakeBackend{}
	r := httptest.NewRequest("POST", "/workflows/validate", strings.NewReader(validRun))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, r)
	var response struct {
		Valid  bool       `json:"valid"`
		Layers [][]string `json:"layers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || !response.Valid || len(response.Layers) != 1 || response.Layers[0][0] != "writer" || b.calls != 0 {
		t.Fatalf("status=%d response=%+v calls=%d", w.Code, response, b.calls)
	}
}

func TestValidateRejectsInvalidDefinition(t *testing.T) {
	for _, body := range []string{`{"nodes":[]}`, validRun + `{}`, `{"nodes":[],"unknown":1}`} {
		b := &fakeBackend{}
		r := httptest.NewRequest("POST", "/workflows/validate", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		testAPI(b).ServeHTTP(w, r)
		if w.Code != 400 || b.calls != 0 {
			t.Fatalf("status=%d calls=%d", w.Code, b.calls)
		}
	}
}
