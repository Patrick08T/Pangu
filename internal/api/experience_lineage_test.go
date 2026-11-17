//go:build integration

package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func experienceLineageRun(t *testing.T, f experienceFixture, purpose string) string {
	t.Helper()
	ctx := identity.WithActor(context.Background(), identity.Actor{ID: f.actor, Namespace: f.namespace})
	item, err := submission.Prepare(ctx, f.namespace, "lineage", uuid.NewString(), submission.Snapshot{Purpose: purpose, Workflow: workflow.Definition{ProjectID: f.project, Input: "must-not-leak-raw-task"}})
	if err != nil {
		t.Fatal(err)
	}
	actual := uuid.NewString()
	if _, err = submission.Dispatch(ctx, f.namespace, item.ID, func(context.Context, *submission.Record) (string, error) { return actual, nil }); err != nil {
		t.Fatal(err)
	}
	return actual
}

func TestExperienceRunLineageUsesActualPersistedBinding(t *testing.T) {
	f := experienceAPI(t)
	run := experienceLineageRun(t, f, "business")
	b := &privateRunBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, project: f.project, purpose: "business"}
	h := New(b, identity.Service{Namespace: f.namespace})
	w := callAuthorized(h, "GET", "/runs/"+run+"/experience", "", f.session)
	if w.Code != 200 {
		t.Fatalf("lineage read: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		ActualRunID string `json:"actual_run_id"`
		Watermark   struct {
			Present    bool  `json:"present"`
			Generation int64 `json:"generation"`
		} `json:"watermark"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ActualRunID != run || !result.Watermark.Present || result.Watermark.Generation != 1 || strings.Contains(w.Body.String(), "must-not-leak") {
		t.Fatalf("binding/summary: %s", w.Body.String())
	}
	for _, query := range []string{"?request_id=" + uuid.NewString(), "?limit=101", "?generation=2"} {
		w = callAuthorized(h, "GET", "/runs/"+run+"/experience"+query, "", f.session)
		if w.Code != 400 {
			t.Fatalf("authority/pagination %s: %d %s", query, w.Code, w.Body.String())
		}
	}
	w = callAuthorized(h, "GET", "/runs/"+uuid.NewString()+"/experience", "", f.session)
	if w.Code != 404 {
		t.Fatalf("unproven actualrun: %d %s", w.Code, w.Body.String())
	}
	private := experienceLineageRun(t, f, "evaluation")
	w = callAuthorized(h, "GET", "/runs/"+private+"/experience", "", f.session)
	if w.Code != 403 && w.Code != 404 {
		t.Fatalf("private actual binding: %d %s", w.Code, w.Body.String())
	}
}

func TestExperienceUnknownPurgedRunCannotExposeLineage(t *testing.T) {
	f := experienceAPI(t)
	run := experienceLineageRun(t, f, "")
	b := &privateRunBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, project: f.project, purpose: "business"}
	w := callAuthorized(New(b, identity.Service{Namespace: f.namespace}), "GET", "/runs/"+run+"/experience", "", f.session)
	if w.Code != 403 && w.Code != 404 {
		t.Fatalf("unknown purged lineage: %d %s", w.Code, w.Body.String())
	}
}

func TestExperienceMemoryObservationRejectsClientResults(t *testing.T) {
	f := experienceAPI(t)
	w := callAuthorized(f.handler, "POST", "/projects/"+f.project+"/experiences/"+uuid.NewString()+"/observe", `{"run_id":"`+uuid.NewString()+`","accepted":true}`, f.session)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "unknown field") {
		t.Fatalf("forged memory observation: %d %s", w.Code, w.Body.String())
	}
}
