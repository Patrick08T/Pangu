package experience

import (
	"testing"
	"time"
)

func TestCandidateRejectsInvalidUTF8BeforeJSONReplacement(t *testing.T) {
	version := sampleVersion("a")
	version.Content.Practice = string([]byte{0xff, 'a'})
	if version.Validate() == nil {
		t.Fatal("invalid UTF8 silently replaced before hashing")
	}
}

func TestServingAlsoRequiresAuthorizedTaskScope(t *testing.T) {
	version := sampleVersion("a")
	version.Canary = DeliveryPolicy{Mode: "serving", TaskScope: []string{"approved"}, TrafficBasisPoints: 10000}
	input := SelectionInput{ProjectID: "p", RunID: "run", RoleID: "builder", Text: "lock", CanaryScope: "not-approved"}
	got, err := SelectCandidates([]Version{version}, input, time.Unix(1000, 0))
	if err != nil || len(got) != 0 {
		t.Fatalf("serving escaped authorization scope: %+v %v", got, err)
	}
}
