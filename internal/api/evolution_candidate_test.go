//go:build integration

package api

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestEvolutionHTTPMemoryBenefitBuildsUnchangedBaseline(t *testing.T) {
	f := experienceAPI(t)
	object, version, memory := uuid.NewString(), uuid.NewString(), uuid.NewString()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1,$2,'agent','baseline','{"agent":{"executor":"eino","instruction":"unchanged-baseline","read_only":true}}')`, object, f.namespace)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) SELECT $1,id,1,1,spec FROM pangu.catalog_objects WHERE id=$2`, version, object)
	if err != nil {
		t.Fatal(err)
	}
	input := evolutionCandidateInput{Kind: "memory_benefit", BenefitMemoryVersionID: memory, BaselineVersion: version}
	request, err := input.candidate(ctx, evolution.Scope{Namespace: f.namespace, ProjectID: f.project, ActorID: f.actor})
	if err != nil {
		t.Fatal(err)
	}
	if request.Kind != "memory_benefit" || request.BenefitMemoryVersionID != memory || len(request.MemoryVersionIDs) != 1 || request.MemoryVersionIDs[0] != memory || request.Payload.Agent.Instruction != "unchanged-baseline" || request.Payload.Agent.Executor != "eino" || request.AgentObjectID != object || request.ProjectID != f.project {
		t.Fatalf("uncontrolled candidate: %+v", request)
	}
}

func TestEvolutionHTTPMemoryBenefitOnlyAcceptsSingleTarget(t *testing.T) {
	f := experienceAPI(t)
	body := map[string]any{"kind": "memory_benefit", "benefit_memory_version_id": uuid.NewString(), "id": uuid.NewString(), "policy_revision": 1, "baseline_version": uuid.NewString(), "environment_id": uuid.NewString(), "suite_id": uuid.NewString(), "budget": map[string]int{"calls": 10, "tokens": 1000}}
	raw, _ := json.Marshal(body)
	path := "/projects/" + f.project + "/evolution/intents"
	w := callAuthorized(f.handler, "POST", path, string(raw), f.session)
	if w.Code != 503 {
		t.Fatalf("valid controlled memory experiment: %d %s", w.Code, w.Body.String())
	}
	for _, field := range []string{"instruction", "memory_version_ids"} {
		body[field] = "forged"
		if field == "memory_version_ids" {
			body[field] = []string{uuid.NewString()}
		}
		raw, _ = json.Marshal(body)
		w = callAuthorized(f.handler, "POST", path, string(raw), f.session)
		if w.Code != 400 {
			t.Fatalf("memory experiment %s: %d %s", field, w.Code, w.Body.String())
		}
		delete(body, field)
	}
}

func TestEvolutionHTTPCandidateRejectsAuthorityAndUnwiredRuntime(t *testing.T) {
	f := experienceAPI(t)
	path := "/projects/" + f.project + "/evolution/intents"
	for _, body := range []string{`{"payload":{"agent":{"executor":"codex"}}}`, `{"passed":true}`, `{"project_id":"` + f.other + `"}`, `{"generation":1,"current":{"hash":"forged"}}`} {
		w := callAuthorized(f.handler, "POST", path, body, f.session)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "unknown field") {
			t.Fatalf("authority write: %d %s", w.Code, w.Body.String())
		}
	}
	body := `{"id":"` + uuid.NewString() + `","policy_revision":1,"baseline_version":"` + uuid.NewString() + `","environment_id":"` + uuid.NewString() + `","suite_id":"` + uuid.NewString() + `","instruction":"仅改Instruction","budget":{"calls":10,"tokens":1000}}`
	w := callAuthorized(f.handler, "POST", path, body, f.session)
	if w.Code != 503 {
		t.Fatalf("unwired candidate runtime: %d %s", w.Code, w.Body.String())
	}
	for _, suffix := range []string{"/decision", "/promote", "/observe"} {
		w = callAuthorized(f.handler, "POST", path+"/"+uuid.NewString()+suffix, `{"generation":1,"passed":true}`, f.session)
		if w.Code != 400 {
			t.Fatalf("%s forged receipt: %d %s", suffix, w.Code, w.Body.String())
		}
	}
}
