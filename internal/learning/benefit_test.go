package learning

import (
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"testing"
)

func TestMemoryCombinationHashUsesImmutableContentNotDelivery(t *testing.T) {
	a := experience.Reference{VersionID: uuid.NewString(), BodyHash: digest([]byte("one")), DeliveryRevision: 1}
	b := experience.Reference{VersionID: uuid.NewString(), BodyHash: digest([]byte("two")), DeliveryRevision: 2}
	one, err := MemoryCombinationHash([]experience.Reference{a, b})
	if err != nil {
		t.Fatal(err)
	}
	a.DeliveryRevision++
	two, err := MemoryCombinationHash([]experience.Reference{b, a})
	if err != nil || one != two {
		t.Fatal("rollout metadata changed measured memory content", err)
	}
	b.BodyHash = digest([]byte("other"))
	changed, _ := MemoryCombinationHash([]experience.Reference{a, b})
	if changed == one {
		t.Fatal("changed memory reused benefit identity")
	}
	if _, err = MemoryCombinationHash([]experience.Reference{a, a}); err == nil {
		t.Fatal("duplicate version accepted")
	}
}
