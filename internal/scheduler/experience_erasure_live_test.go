//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func erasureBackendHTTP(t *testing.T, status string) Backend {
	t.Helper()
	workflowID := uuid.NewString()
	t.Setenv("PANGU_WORKER_ID", "test")
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	return maintenanceBackendHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/workflows") {
			_ = json.NewEncoder(w).Encode(map[string]any{"rows": []any{map[string]any{"name": r.URL.Query().Get("name"), "metadata": map[string]any{"id": workflowID}}}})
			return
		}
		if status == "404" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"run": map[string]any{"metadata": map[string]any{"id": path.Base(r.URL.Path)}, "workflowId": workflowID, "status": status}, "tasks": []any{}, "taskEvents": []any{}})
	}))
}

func TestExperienceErasureLiveRunningOverridesArchivedStop(t *testing.T) {
	b := erasureBackendHTTP(t, "RUNNING")
	f := newErasureFixtureNamespace(t, "ttl", b.Namespace())
	f.usage(t, f.request, true)
	f.runCopies(t)
	f.stopReceipt(t, f.run)
	raw, err := getHatchetRun(context.Background(), f.run)
	if err != nil || runStatus(raw.Run.Status) != "running" {
		t.Fatal("external running fixture was not decoded", err)
	}
	phase, err := b.experienceEraseRun(context.Background(), experienceErasureBinding{f.ns, f.project, f.request, f.run})
	var purged bool
	if readErr := f.conn.QueryRow(context.Background(), `SELECT purged_at IS NOT NULL FROM pangu.submissions WHERE id=$1`, f.request).Scan(&purged); readErr != nil {
		t.Fatal(readErr)
	}
	if err == nil || purged || phase != "WaitingStop" {
		t.Fatalf("old archive covered contradictory live execution: %s %v %v", phase, err, purged)
	}
}
