package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestRegistrationScansFuzzyMatchPages(t *testing.T) {
	id := uuid.NewString()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		rows := []map[string]any{}
		if r.URL.Query().Get("offset") == "0" {
			for range 100 {
				rows = append(rows, map[string]any{"name": "other-pangu", "metadata": map[string]any{"id": uuid.NewString()}})
			}
		} else {
			rows = append(rows, map[string]any{"name": "pangu", "metadata": map[string]any{"id": id}})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows})
	}))
	defer server.Close()
	t.Setenv("HATCHET_CLIENT_SERVER_URL", server.URL)
	t.Setenv("HATCHET_CLIENT_TOKEN", "test-token")
	t.Setenv("HATCHET_CLIENT_TENANT_ID", uuid.NewString())
	got, err := registeredWorkflowID(context.Background(), "pangu")
	if err != nil || got.String() != id || calls != 2 {
		t.Fatalf("漏掉后页精确匹配: %s %v calls=%d", got, err, calls)
	}
}
