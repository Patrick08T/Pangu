package api

import (
	"encoding/json"
	contract "github.com/Patrick08T/Pangu/api"
	"github.com/getkin/kin-openapi/openapi3"
	"testing"
)

func TestExperienceOpenAPIContract(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData(contract.OpenAPI)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/projects/{id}/experience-policy", "/projects/{id}/experience-capabilities", "/projects/{id}/experiences", "/projects/{id}/experiences/{version}", "/projects/{id}/experiences/{version}/usages", "/projects/{id}/evolution/intents", "/projects/{id}/evolution/intents/{intent}/rollback", "/runs/{id}/experience", "/projects/{id}/experience-artifacts/{artifact}", "/runs/{id}/rebuild-intents", "/projects/{id}/experiences/{version}/observe", "/projects/{id}/entries/{entry}/runs"} {
		if doc.Paths.Find(path) == nil {
			t.Errorf("missing contract %s", path)
		}
	}
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"ExperiencePolicyUpdate", `{"expected_revision":0,"policy":{"enabled":false}}`, true},
		{"ExperiencePolicyUpdate", `{"policy":{"enabled":false}}`, false},
		{"ExperiencePolicyUpdate", `{"expected_revision":0,"policy":{"actor_id":"forged"}}`, false},
		{"ExperienceClaimRequest", `{"checker_id":"00000000-0000-0000-0000-000000000001","intent_id":"00000000-0000-0000-0000-000000000002","generation":1}`, true},
		{"ExperienceClaimRequest", `{"receipt_id":"forged","passed":true}`, false},
		{"EvolutionRollback", `{"generation":1,"reason":"rollback","score":1}`, false},
		{"ExperienceObservationRequest", `{"run_id":"00000000-0000-0000-0000-000000000001"}`, true},
		{"ExperienceObservationRequest", `{"run_id":"00000000-0000-0000-0000-000000000001","accepted":true}`, false},
		{"WorkbenchEntrySubmission", `{"input":"business task"}`, true},
		{"WorkbenchEntrySubmission", `{"input":"business task","version_id":"forged"}`, false},
		{"WorkbenchEntrySubmission", `{"input":"business task","entry_revision":1}`, false},
	} {
		schema := doc.Components.Schemas[test.name]
		if schema == nil {
			t.Errorf("missing schema %s", test.name)
			continue
		}
		var value any
		if err = json.Unmarshal([]byte(test.body), &value); err != nil {
			t.Fatal(err)
		}
		if err = schema.Value.VisitJSON(value); (err == nil) != test.valid {
			t.Errorf("%s: %v", test.name, err)
		}
	}
}
