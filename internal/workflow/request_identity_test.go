package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNodeInputTransportsSubmissionIdentityOutsideLogicalHash(t *testing.T) {
	var first, second NodeInput
	if err := json.Unmarshal([]byte(`{"request_id":"request-one","node":{"id":"n"},"input":"same"}`), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"request_id":"request-two","node":{"id":"n"},"input":"same"}`), &second); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(first)
	if err != nil || !strings.Contains(string(encoded), `"request_id":"request-one"`) {
		t.Fatalf("worker lost accepted request identity: %s %v", encoded, err)
	}
	one, err := EffectiveInputHash(first)
	if err != nil {
		t.Fatal(err)
	}
	two, err := EffectiveInputHash(second)
	if err != nil || one != two {
		t.Fatalf("physical submission IDs prevent unchanged logical reuse: %s %s %v", one, two, err)
	}
}
