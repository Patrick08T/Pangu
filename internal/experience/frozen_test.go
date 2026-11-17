package experience

import (
	"testing"
	"time"
)

func TestFrozenSelectionCannotSeeLaterPromotionOrNewVersions(t *testing.T) {
	version, later := sampleVersion("a"), sampleVersion("b")
	version.BodyHash, later.BodyHash = bodyHash(version.Content), bodyHash(later.Content)
	mark := Watermark{Versions: []FrozenVersion{{Reference: Reference{VersionID: "a", BodyHash: version.BodyHash, DeliveryRevision: 1}, Delivery: "shadow"}}}
	input := SelectionInput{ProjectID: "p", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	got, err := SelectFrozenCandidates([]Version{version, later}, mark, input, time.Unix(1000, 0))
	if err != nil || len(got) != 0 {
		t.Fatalf("later promotion leaked: %+v %v", got, err)
	}
}

func TestFrozenSelectionPreservesSupersededVersionButNotPause(t *testing.T) {
	version := sampleVersion("a")
	version.BodyHash, version.Validity = bodyHash(version.Content), "superseded"
	mark := Watermark{Versions: []FrozenVersion{{Reference: Reference{VersionID: "a", BodyHash: version.BodyHash, DeliveryRevision: 1}, Delivery: "serving", Canary: version.Canary}}}
	input := SelectionInput{ProjectID: "p", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	got, err := SelectFrozenCandidates([]Version{version}, mark, input, time.Unix(1000, 0))
	if err != nil || len(got) != 1 {
		t.Fatalf("supersession rewrote frozen input: %+v %v", got, err)
	}
	version.Delivery = "paused"
	got, err = SelectFrozenCandidates([]Version{version}, mark, input, time.Unix(1000, 0))
	if err != nil || len(got) != 0 {
		t.Fatalf("pause ignored for fresh selection: %+v %v", got, err)
	}
	version.Delivery, version.BodyHash = "serving", "tampered"
	if _, err := SelectFrozenCandidates([]Version{version}, mark, input, time.Unix(1000, 0)); err == nil {
		t.Fatal("frozen content mismatch ignored")
	}
}
