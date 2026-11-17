package scheduler

import (
	"encoding/json"
	"testing"
)

func TestRunPurposeSurvivesPrivateHistoryWithoutPublicDisclosure(t *testing.T) {
	for _, purpose := range []string{"evaluation", "learning", "business"} {
		item := RunDetails{RunSummary: RunSummary{ID: "run", Purpose: purpose}, LogsExpired: true}
		raw, err := marshalRunHistory(item)
		if err != nil {
			t.Fatal(err)
		}
		got, err := unmarshalRunHistory(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got.Purpose != purpose {
			t.Fatalf("history lost protected purpose %q: %s", purpose, raw)
		}
		public, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(public, &fields)
		if _, ok := fields["internal_purpose"]; ok {
			t.Fatal("private routing metadata disclosed")
		}
	}
}
