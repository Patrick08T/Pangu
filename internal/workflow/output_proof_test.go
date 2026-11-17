package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOutputPreservesLineageWithoutChangingPayloadFingerprint(t *testing.T) {
	original := NodeOutput{Text: "verified output", RunID: "attempt", ContentRevision: "commit"}
	hash, err := OutputFingerprint(original)
	if err != nil {
		t.Fatal(err)
	}
	data := `{"text":"verified output","run_id":"attempt","content_revision":"commit","lineage":{"version":1,"artifact_id":"artifact","input_hash":"input","output_hash":"` + hash + `"}}`
	var output NodeOutput
	if err = json.Unmarshal([]byte(data), &output); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(output)
	if err != nil || !strings.Contains(string(encoded), `"artifact_id":"artifact"`) {
		t.Fatalf("serialized checkpoint lost its lineage: %s %v", encoded, err)
	}
	actual, err := OutputFingerprint(output)
	if err != nil || actual != hash {
		t.Fatalf("lineage envelope changed payload hash: %s %v", actual, err)
	}
}
