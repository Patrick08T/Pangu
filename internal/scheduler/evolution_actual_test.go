package scheduler

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func actualMemoryFixture() workflow.Experience {
	content := experience.Content{Kind: "practice", Observation: "JSON fields", Practice: "Check required keys", Conditions: "JSON task", Counterexamples: "Not prose", Applicability: experience.Applicability{Roles: []string{uuid.NewString()}}}
	return workflow.Experience{Reference: workflow.MemoryReference{VersionID: uuid.NewString(), BodyHash: evolutionHash(content), DeliveryRevision: 1}, Kind: content.Kind, Conditions: content.Conditions, Content: evolutionJSON(content)}
}

func TestEvolutionActualMemoryHashVerifiesInjectedBody(t *testing.T) {
	memory := actualMemoryFixture()
	original, err := evolutionActualMemoryHash([]workflow.Experience{memory})
	if err != nil || original == evaluation.EmptyMemoryHash {
		t.Fatal(original, err)
	}
	memory.Reference.DeliveryRevision++
	later, err := evolutionActualMemoryHash([]workflow.Experience{memory})
	if err != nil || later != original {
		t.Fatal("delivery revision changed body combination", later, err)
	}
	for _, name := range []string{"body", "reference", "metadata", "unknown_json", "duplicate_json", "case_alias"} {
		t.Run(name, func(t *testing.T) {
			bad := actualMemoryFixture()
			switch name {
			case "body":
				bad.Content = []byte(`{"kind":"practice","practice":"different body"}`)
			case "reference":
				bad.Reference.BodyHash = evolutionTextHash("not body")
			case "metadata":
				bad.Conditions = "different rendered conditions"
			case "unknown_json":
				bad.Content = append(append([]byte{}, bad.Content[:len(bad.Content)-1]...), []byte(`,"extra":"model-visible text"}`)...)
			case "duplicate_json":
				bad.Content = append(append([]byte{}, bad.Content[:len(bad.Content)-1]...), []byte(`,"kind":"practice"}`)...)
			case "case_alias":
				bad.Content = append(append([]byte{}, bad.Content[:len(bad.Content)-1]...), []byte(`,"Kind":"practice"}`)...)
			}
			if _, e := evolutionActualMemoryHash([]workflow.Experience{bad}); e == nil {
				t.Fatal("unverified actual memory accepted")
			}
		})
	}
}
