package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRequestTracingDoesNotLogUserContent(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	r := httptest.NewRequest("GET", "/runs/private-secret?token=query-secret", nil)
	r.Header.Set("X-Request-ID", "caller-secret")
	r.Header.Set("Authorization", "Bearer credential-secret")
	w := httptest.NewRecorder()
	testAPI(&fakeBackend{}).ServeHTTP(w, r)
	id := w.Header().Get("X-Request-ID")
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("missing generated request ID: %q", id)
	}
	var event map[string]any
	if err := json.Unmarshal(logs.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event["request_id"] != id || event["route"] != "GET /runs/{id}" || event["status"] != float64(400) {
		t.Fatalf("event=%v", event)
	}
	if strings.Contains(logs.String(), "secret") {
		t.Fatal("request details leaked into access log")
	}
}

func TestRequestIDsAreUnique(t *testing.T) {
	h := testAPI(&fakeBackend{})
	first, second := httptest.NewRecorder(), httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest("GET", "/healthz", nil))
	h.ServeHTTP(second, httptest.NewRequest("GET", "/missing", nil))
	if first.Header().Get("X-Request-ID") == second.Header().Get("X-Request-ID") {
		t.Fatal("requests must have distinct generated IDs, including 404 responses")
	}
}
