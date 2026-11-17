package experience

import (
	"encoding/json"
	"testing"
)

func TestPolicyJSONDistinguishesOmittedBoundsFromExplicitZero(t *testing.T) {
	data, err := json.Marshal(enabledPolicy())
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	evaluation := object["evaluation"].(map[string]any)
	for _, key := range []string{"non_inferiority", "resource_tolerance"} {
		original := evaluation[key]
		delete(evaluation, key)
		missing, _ := json.Marshal(object)
		var policy Policy
		if err := json.Unmarshal(missing, &policy); err != nil {
			t.Fatal(err)
		}
		if policy.Validate() == nil {
			t.Fatalf("omitted %s inferred as authorized zero", key)
		}
		evaluation[key] = float64(0)
		explicit, _ := json.Marshal(object)
		if err := json.Unmarshal(explicit, &policy); err != nil {
			t.Fatal(err)
		}
		if err := policy.Validate(); err != nil {
			t.Fatalf("explicit zero %s rejected: %v", key, err)
		}
		evaluation[key] = original
	}
}
