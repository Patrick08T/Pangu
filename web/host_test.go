package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSameOriginServesOnlyBuiltAssetsAndAPIPrefix(t *testing.T) {
	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("API 前缀未移除: %s", r.URL.Path)
		}
		w.Write([]byte(`{"status":"ok"}`))
	}))
	for _, item := range []struct {
		path     string
		status   int
		contains string
	}{{"/", 200, "id=\"root\""}, {"/api/healthz", 200, `"status":"ok"`}, {"/healthz", 404, ""}, {"/src/App.tsx", 404, ""}, {"/assets/", 404, ""}, {"/.env", 404, ""}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", item.path, nil))
		if response.Code != item.status || !strings.Contains(response.Body.String(), item.contains) {
			t.Errorf("%s: %d %s", item.path, response.Code, response.Body.String())
		}
		if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("缺少安全响应头: %s", item.path)
		}
	}
}
