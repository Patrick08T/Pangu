package experience

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func sampleVersion(id string) Version {
	return Version{ID: id, EntryID: id, Revision: 1, ProjectID: "p", Validity: "active",
		Delivery: "serving", DeliveryRevision: 1, ExpiresAt: time.Unix(2000, 0),
		Canary: DeliveryPolicy{Mode: "serving", TaskScope: []string{"approved"}, TrafficBasisPoints: 10000},
		Content: Content{Kind: "practice", Observation: "a lock fails", Practice: "retry only after stop",
			Conditions: "only isolated test", Counterexamples: "not authorization failures",
			Applicability: Applicability{Roles: []string{"builder"}, Keywords: []string{"lock"}}},
		Sources: []Source{{ID: "source", ProjectID: "p", Kind: "report", Hash: strings.Repeat("a", 64)}}}
}

func TestCandidateValidationRequiresBoundedSourcedClaims(t *testing.T) {
	version := sampleVersion("a")
	if err := version.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Version){
		"oversized UTF8": func(v *Version) { v.Content.Practice = strings.Repeat("界", 700) },
		"missing source": func(v *Version) { v.Sources = nil },
		"foreign source": func(v *Version) { v.Sources[0].ProjectID = "other" },
		"unbound source": func(v *Version) { v.Sources[0].Hash = "not-a-hash" },
		"missing limits": func(v *Version) { v.Content.Counterexamples = "" },
		"missing kind":   func(v *Version) { v.Content.Kind = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := sampleVersion("a")
			change(&candidate)
			if candidate.Validate() == nil {
				t.Fatal("invalid claim accepted")
			}
		})
	}
}

func TestSelectCandidatesFiltersBeforeDeterministicRanking(t *testing.T) {
	versions := []Version{sampleVersion("b"), sampleVersion("a"), sampleVersion("foreign"), sampleVersion("shadow"), sampleVersion("expired"), sampleVersion("wrong-role")}
	versions[2].ProjectID, versions[3].Delivery = "other", "shadow"
	versions[4].ExpiresAt = time.Unix(500, 0)
	versions[5].Content.Applicability.Roles = []string{"reviewer"}
	input := SelectionInput{ProjectID: "p", RunID: "run", RoleID: "builder", Text: "lock failure", CanaryScope: "approved"}
	got, err := SelectCandidates(versions, input, time.Unix(1000, 0))
	if err != nil || len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("filtered selection = %+v, %v", got, err)
	}
	input.Text = "unrelated"
	got, err = SelectCandidates(versions, input, time.Unix(1000, 0))
	if err != nil || len(got) != 0 {
		t.Fatalf("unreliable match injected: %+v %v", got, err)
	}
}

func TestSelectCandidatesLimitsCountAndRejectsInvalidPayload(t *testing.T) {
	versions := []Version{}
	for _, id := range []string{"f", "e", "d", "c", "b", "a"} {
		versions = append(versions, sampleVersion(id))
	}
	input := SelectionInput{ProjectID: "p", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	got, err := SelectCandidates(versions, input, time.Unix(1000, 0))
	if err != nil || len(got) != 4 || got[3].ID != "d" {
		t.Fatalf("bounded result: %+v %v", got, err)
	}
	versions[0].Content.Practice = strings.Repeat("x", 2049)
	if _, err = SelectCandidates(versions, input, time.Unix(1000, 0)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid source payload disguised as empty match: %v", err)
	}
}

func TestCanaryGroupingRequiresScopeAndDoesNotChangeOnRetry(t *testing.T) {
	version := sampleVersion("a")
	version.Delivery = "canary"
	version.Canary = DeliveryPolicy{Mode: "canary", TaskScope: []string{"approved"}, TrafficBasisPoints: 10000}
	input := SelectionInput{ProjectID: "p", RunID: "fixed", RoleID: "builder", Text: "lock"}
	got, err := SelectCandidates([]Version{version}, input, time.Unix(1000, 0))
	if err != nil || len(got) != 0 {
		t.Fatalf("unauthorized canary: %+v %v", got, err)
	}
	input.CanaryScope = "approved"
	for range 3 {
		got, err = SelectCandidates([]Version{version}, input, time.Unix(1000, 0))
		if err != nil || len(got) != 1 {
			t.Fatalf("canary group changed: %+v %v", got, err)
		}
	}
}
