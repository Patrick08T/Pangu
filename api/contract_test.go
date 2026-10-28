package contract

import (
	"context"
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
	for _, path := range []string{"/healthz", "/readyz", "/openapi.yaml", "/workflows/validate", "/runs", "/runs/{id}", "/crons", "/crons/{id}"} {
		if document.Paths.Find(path) == nil {
			t.Errorf("missing contract for %s", path)
		}
	}
}
