//go:build integration

package scheduler

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func maintenanceHistoryRun(t *testing.T, b Backend, project string, archive bool) string {
	t.Helper()
	ctx := context.Background()
	record, err := submission.Prepare(ctx, b.Namespace(), "maintenance-preview", uuid.NewString(), submission.Snapshot{Purpose: "business", Workflow: workflow.Definition{ProjectID: project, Input: "retained business input"}})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, b.Namespace(), record.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	entryExec(t, `UPDATE pangu.submissions SET created_at=clock_timestamp()-interval '32 days' WHERE id=$1`, record.ID)
	if archive {
		finished := time.Now().Add(-31 * 24 * time.Hour)
		body, err := marshalRunHistory(RunDetails{RunSummary: RunSummary{ID: run, ProjectID: project, Purpose: "business", Status: "succeeded", FinishedAt: &finished}, StopConfirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = submission.SaveHistory(ctx, b.Namespace(), run, finished, body); err != nil {
			t.Fatal(err)
		}
	}
	return run
}

func maintenancePreviewSnapshot(t *testing.T, namespace string) string {
	t.Helper()
	var value string
	err := entryQuery(t, `SELECT jsonb_build_object(
 'history',(SELECT jsonb_agg(to_jsonb(h) ORDER BY h.run_id) FROM pangu.run_history h WHERE namespace=$1),
 'submissions',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM pangu.submissions s WHERE namespace=$1),
 'retention',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.run_id) FROM pangu.retention_records r WHERE namespace=$1),
 'versions',(SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id) FROM pangu.experience_versions v WHERE namespace=$1),
 'bodies',(SELECT jsonb_agg(to_jsonb(b) ORDER BY b.version_id) FROM pangu.experience_bodies b JOIN pangu.experience_versions v ON v.id=b.version_id WHERE v.namespace=$1),
 'states',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.version_id) FROM pangu.experience_states s JOIN pangu.experience_versions v ON v.id=s.version_id WHERE v.namespace=$1),
 'checkers',(SELECT jsonb_agg(to_jsonb(b) ORDER BY b.checker_id) FROM pangu.learning_checker_bodies b JOIN pangu.learning_checkers c ON c.id=b.checker_id WHERE c.namespace=$1),
 'audit',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM pangu.audit_events a WHERE namespace=$1)
 )::text`, namespace).Scan(&value)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestExperienceMaintenancePreviewReadsOnlyPersistedHistory(t *testing.T) {
	entryDB(t)
	var calls atomic.Int64
	b := maintenanceBackendHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	ns, project := b.Namespace(), uuid.NewString()
	maintenanceProject(t, ns, project)
	entryExec(t, `UPDATE pangu.projects SET worker_id=$2 WHERE id=$1`, project, workspace.WorkerID())
	known := maintenanceHistoryRun(t, b, project, true)
	unknown := maintenanceHistoryRun(t, b, project, false)
	maintenanceMemory(t, ns, project, true, true, true)
	maintenanceChecker(t, ns, project)
	before := maintenancePreviewSnapshot(t, ns)
	items, err := b.Retention(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, item := range items {
		states[item.RunID] = item.Status
	}
	if states[known] != "eligible" || states[unknown] != "protected" {
		t.Fatalf("preview did not use conservative stored proof: %v", states)
	}
	if calls.Load() != 0 {
		t.Fatalf("preview refreshed external authority: %d calls", calls.Load())
	}
	if after := maintenancePreviewSnapshot(t, ns); after != before {
		t.Fatal("preview mutated history or cleanup state")
	}
	items, err = b.retainWorkspaces(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() == 0 {
		t.Fatal("real retention trusted stale history instead of checking live state")
	}
	for _, item := range items {
		if item.Status != "protected" {
			t.Fatalf("unavailable current proof allowed cleanup: %+v", item)
		}
	}
	var purged bool
	if err = entryQuery(t, `SELECT EXISTS(SELECT 1 FROM pangu.submissions WHERE namespace=$1 AND purged_at IS NOT NULL)`, ns).Scan(&purged); err != nil || purged {
		t.Fatalf("stale preview proof erased actual payload: %v %v", purged, err)
	}
}
