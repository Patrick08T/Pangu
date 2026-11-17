package experience

import (
	"testing"
	"time"
)

func TestCurrentPolicyTrafficRestrictionOverridesFrozenDelivery(t *testing.T) {
	policy := enabledPolicy()
	policy.Roles = []string{"builder"}
	policy.Delivery = DeliveryPolicy{Mode: "serving", TaskScope: []string{"approved"}, TrafficBasisPoints: 1}
	input := SelectionInput{ProjectID: "p", RunID: "not-approved", RoleID: "builder", CanaryScope: "approved"}
	if policyAllowsInput(policy, input) {
		t.Fatal("newly restricted cohort still authorized")
	}
}

func TestServingCannotExpandAuthorizedTrafficPercentage(t *testing.T) {
	version := sampleVersion("a")
	version.Canary.TrafficBasisPoints = 1
	input := SelectionInput{ProjectID: "p", RunID: "not-approved", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	got, err := SelectCandidates([]Version{version}, input, time.Unix(1000, 0))
	if err != nil || len(got) != 0 {
		t.Fatalf("serving expanded restricted cohort: %+v %v", got, err)
	}
}
