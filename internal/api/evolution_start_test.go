//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/identity"
)

func (f evolutionStartAPIFixture) effects(t *testing.T) [4]int {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var out [4]int
	err = conn.QueryRow(context.Background(), `SELECT
 (SELECT count(*) FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2),
 (SELECT count(*) FROM pangu.evolution_budget_charges WHERE namespace=$1 AND project_id=$2),
 (SELECT count(*) FROM pangu.evolution_target_claims WHERE namespace=$1 AND project_id=$2),
 (SELECT count(*) FROM pangu.evolution_executions e JOIN pangu.evolution_intents i ON i.id=e.intent_id WHERE i.namespace=$1 AND i.project_id=$2)`, f.namespace, f.project).Scan(&out[0], &out[1], &out[2], &out[3])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEvolutionStartHTTPReturnsDurableSafeAcceptance(t *testing.T) {
	f := newEvolutionStartAPI(t)
	path := "/projects/" + f.project + "/evolution/intents"
	raw, _ := json.Marshal(f.input)
	first := callAuthorized(f.handler, "POST", path, string(raw), f.session)
	if first.Code != http.StatusAccepted {
		t.Fatalf("real start: %d %s", first.Code, first.Body.String())
	}
	var result evolution.IntentSummary
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != f.input.ID || result.ProjectID != f.project || result.State != "evaluating" || result.Generation != 1 || result.ExperimentID == "" || result.Publication != nil {
		t.Fatalf("acceptance must describe durable pending work, not completion: %+v", result)
	}
	for _, marker := range []string{"candidate-private-instruction", "baseline-private-instruction", "checker-only-value", "private case", `"payload"`, `"request"`, `"suite"`, `"receipt"`, "test-not-a-secret"} {
		if strings.Contains(first.Body.String(), marker) {
			t.Fatalf("private payload in public acceptance: %s", marker)
		}
	}
	second := callAuthorized(f.handler, "POST", path, string(raw), f.session)
	if second.Code != http.StatusAccepted || second.Body.String() != first.Body.String() {
		t.Fatalf("same request was not idempotent: %d %s", second.Code, second.Body.String())
	}
	read := callAuthorized(f.handler, "GET", path+"/"+result.ID, "", f.session)
	if read.Code != 200 || read.Body.String() != first.Body.String() {
		t.Fatalf("accepted work not durably readable: %d %s", read.Code, read.Body.String())
	}
	f.input.Instruction = "conflicting candidate"
	raw, _ = json.Marshal(f.input)
	conflict := callAuthorized(f.handler, "POST", path, string(raw), f.session)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed same-id request: %d %s", conflict.Code, conflict.Body.String())
	}
	if effects := f.effects(t); effects != [4]int{1, 1, 3, 0} || f.calls.Load() != 0 {
		t.Fatalf("start synchronously drove or charged twice: %v HTTP=%d", effects, f.calls.Load())
	}
}

func TestEvolutionStartHTTPCapabilityReflectsRealAdapter(t *testing.T) {
	f := newEvolutionStartAPI(t)
	if got := capabilityRead(t, f.handler, f.experienceFixture); !got.Features["evolution"] {
		t.Fatal("real wired Backend remains unavailable to the experience console")
	}
	unwired := New(&authorizedBackend{namespace: f.namespace}, identity.Service{Namespace: f.namespace})
	if got := capabilityRead(t, unwired, f.experienceFixture); got.Features["evolution"] {
		t.Fatal("deliberately unimplemented backend claims evolution readiness")
	}
	if effects := f.effects(t); effects != [4]int{} || f.calls.Load() != 0 {
		t.Fatalf("capability read started work: %v HTTP=%d", effects, f.calls.Load())
	}
}

func TestEvolutionStartHTTPRejectsTokenAndCurrentAuthorityLoss(t *testing.T) {
	for _, name := range []string{"operator", "disabled_policy", "revoked_policy", "foreign_project", "token"} {
		t.Run(name, func(t *testing.T) {
			f := newEvolutionStartAPI(t)
			path := "/projects/" + f.project + "/evolution/intents"
			if name == "disabled_policy" || name == "revoked_policy" {
				f.input.PolicyRevision = 2
			}
			raw, _ := json.Marshal(f.input)
			var response *httptest.ResponseRecorder
			switch name {
			case "operator":
				f.role(t, "operator")
			case "disabled_policy":
				capabilityExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT namespace,project_id,2,actor_id,jsonb_set(jsonb_set(policy,'{revision}','2'),'{enabled}','false') FROM pangu.experience_policies WHERE namespace=$1 AND revision=1`, f.namespace)
			case "revoked_policy":
				capabilityExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT namespace,project_id,2,actor_id,jsonb_set(jsonb_set(policy,'{revision}','2'),'{revoked}','true') FROM pangu.experience_policies WHERE namespace=$1 AND revision=1`, f.namespace)
			case "foreign_project":
				path = "/projects/" + f.other + "/evolution/intents"
			case "token":
				actor := identity.Actor{ID: f.actor, Namespace: f.namespace}
				_, token, err := identity.IssueToken(context.Background(), actor, f.project, "start forbidden", []string{"read", "run"}, time.Now().Add(time.Hour))
				if err != nil {
					t.Fatal(err)
				}
				r := httptest.NewRequest("POST", path, strings.NewReader(string(raw)))
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("Content-Type", "application/json")
				response = httptest.NewRecorder()
				f.handler.ServeHTTP(response, r)
			}
			if response == nil {
				response = callAuthorized(f.handler, "POST", path, string(raw), f.session)
			}
			if response.Code != http.StatusForbidden || f.effects(t) != [4]int{} || f.calls.Load() != 0 {
				t.Fatalf("unauthorized start created effects: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
