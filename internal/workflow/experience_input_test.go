package workflow

import (
	"encoding/json"
	"testing"
)

func TestNodeInputRoundTripsStructuredExperiences(t *testing.T) {
	var input NodeInput
	raw := []byte(`{"experiences":[{"reference":{"version_id":"version-a","body_hash":"body-digest","delivery_revision":7},"kind":"heuristic","conditions":"timeout only","content":{"practice":"verify the timeout source"},"sources":[{"id":"evidence-a","kind":"command","run_id":"source-run","node_id":"verify","evidence_hash":"source-digest"}],"reason":"matched timeout"}]}`)
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Experiences []struct {
			Reference map[string]any   `json:"reference"`
			Content   map[string]any   `json:"content"`
			Sources   []map[string]any `json:"sources"`
			Reason    string           `json:"reason"`
		} `json:"experiences"`
	}
	if err = json.Unmarshal(actual, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Experiences) != 1 {
		t.Fatalf("durable model input lost frozen experiences: %s", actual)
	}
	e := got.Experiences[0]
	if e.Content["practice"] != "verify the timeout source" || e.Reference["delivery_revision"] != float64(7) || len(e.Sources) != 1 || e.Sources[0]["evidence_hash"] != "source-digest" || e.Reason != "matched timeout" {
		t.Fatalf("experience input lost content/provenance: %+v", e)
	}
}
