package contract

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPIContract(t *testing.T) {
	document, err := openapi3.NewLoader().LoadFromData(OpenAPI)
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/healthz", "/readyz", "/openapi.yaml", "/workflows/validate", "/runs", "/runs/{id}", "/crons", "/crons/{id}", "/catalog/{id}/trials", "/catalog/{id}/draft/preflight", "/models/codex", "/factory/sessions", "/factory/sessions/{id}", "/factory/sessions/{id}/messages", "/factory/sessions/{id}/apply"} {
		if document.Paths.Find(path) == nil {
			t.Errorf("missing contract for %s", path)
		}
	}
}

func TestCodexAccountConnectionContract(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData(OpenAPI)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		kind, raw string
		valid     bool
	}{
		{"Connection", `{"provider":"codex","model":""}`, true},
		{"Connection", `{"provider":"codex"}`, true},
		{"Connection", `{"provider":"codex","base_url":"https://example.com"}`, false},
		{"Connection", `{"provider":"codex","secret_ref":"PANGU_SECRET_TEST"}`, false},
		{"Connection", `{"provider":"unknown"}`, false},
		{"Connection", `{"base_url":"https://example.com","model":"test","secret_ref":"PANGU_SECRET_TEST"}`, true},
		{"Node", `{"id":"pm","executor":"eino","instruction":"规划","connection_version":"00000000-0000-0000-0000-000000000001"}`, true},
		{"Node", `{"id":"pm","executor":"eino","instruction":"规划"}`, false},
		{"Node", `{"id":"pm","executor":"eino","instruction":"规划","model":"test"}`, true},
	} {
		var value any
		if err := json.Unmarshal([]byte(test.raw), &value); err != nil {
			t.Fatal(err)
		}
		err := doc.Components.Schemas[test.kind].Value.VisitJSON(value)
		if (err == nil) != test.valid {
			t.Errorf("%s 验证结果不符: %s %v", test.kind, test.raw, err)
		}
	}
}
