//go:build integration

package api

import (
	"context"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestExperienceScopedCatalogReads(t *testing.T) {
	f := experienceAPI(t)
	object, version := uuid.NewString(), uuid.NewString()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1,$2,'agent','baseline','{"agent":{"executor":"codex","instruction":"private-marker","read_only":true}}')`, object, f.namespace); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(context.Background(), `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) SELECT $1,id,1,1,spec FROM pangu.catalog_objects WHERE id=$2`, version, object); err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{"/runnables", "/versions/" + version, "/catalog/" + object + "/versions"} {
		w := callAuthorized(f.handler, "GET", "/projects/"+f.project+tail, "", f.session)
		if w.Code != 200 || !strings.Contains(w.Body.String(), version) {
			t.Fatalf("%s: %d %s", tail, w.Code, w.Body.String())
		}
		w = callAuthorized(f.handler, "GET", "/projects/"+f.other+tail, "", f.session)
		if w.Code != 403 || strings.Contains(w.Body.String(), "private-marker") {
			t.Fatalf("cross scope: %d %s", w.Code, w.Body.String())
		}
	}
	w := callAuthorized(f.handler, "GET", "/projects/"+f.project+"/runnables?allow_observing=true", "", f.session)
	if w.Code != 400 {
		t.Fatalf("authority query: %d %s", w.Code, w.Body.String())
	}
}

func TestExperienceSubmissionRejectsPrivilegedFields(t *testing.T) {
	f := experienceAPI(t)
	for _, field := range []string{"purpose", "evaluation_environment", "experiences", "memory_watermark", "lineage", "experiment_id"} {
		body := `{"project_id":"` + f.project + `","` + field + `":"forged"}`
		w := callAuthorized(f.handler, "POST", "/runs", body, f.session)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "unknown field") {
			t.Fatalf("%s: %d %s", field, w.Code, w.Body.String())
		}
	}
}
