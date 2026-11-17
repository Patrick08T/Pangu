//go:build integration

package learning

import (
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func claimContent() experience.Content {
	return experience.Content{Kind: "rule", Observation: "Original JSON contains a validated result", Practice: "Check the result.valid contract",
		Conditions: "Only for reports with the registered JSON contract", Counterexamples: "Prose is not verified",
		Applicability: experience.Applicability{Roles: []string{"researcher"}, Keywords: []string{"JSON"}}}
}

func (f learningFixture) claim(t *testing.T, policies ...experience.Policy) (experience.Version, experience.LearningIntent, Checker) {
	t.Helper()
	var policy experience.Policy
	if len(policies) == 0 {
		policy = f.policy(t)
	} else {
		policy = policies[0]
	}
	source := f.report(t)
	if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	checker, err := RegisterChecker(f.ctx, f.scope, CheckerSpec{Kind: "json_assertion_v1", ContentHash: jsonHash(claimContent()),
		SourceKind: "report", NodeID: source.NodeID, JSONPointer: "/result/valid", Expected: []byte(`true`)}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	intent, err := f.store.EnsureLearningIntent(f.ctx, f.scope, experience.IntentInput{SourceRunID: source.RunID,
		EvidenceRevision: 1, PolicyRevision: policy.Revision, Purpose: "business", Terminal: true, StopConfirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	version, err := f.store.Create(f.ctx, f.scope, experience.Version{ProjectID: f.scope.ProjectID, Revision: 1, Content: claimContent(), Sources: []experience.Source{source}, ExpiresAt: time.Now().Add(30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	return version, intent, checker
}

func TestLearningClaimReceiptCommitsWithActivation(t *testing.T) {
	f := newLearningFixture(t)
	version, intent, checker := f.claim(t)
	request, err := CheckClaim(f.ctx, f.scope, version.ID, checker.ID, intent.ID, intent.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadReceipt(f.ctx, f.scope, request.ReceiptID); !errors.Is(err, experience.ErrNotFound) {
		t.Fatal("pending request pretended to be receipt", err)
	}
	active, err := f.store.Activate(f.ctx, f.scope, version.ID, request)
	if err != nil || active.Validity != "active" || active.Delivery != "shadow" {
		t.Fatal(active, err)
	}
	receipt, err := ReadReceipt(f.ctx, f.scope, request.ReceiptID)
	if err != nil || !receipt.Passed || receipt.Hash == "" {
		t.Fatal(receipt, err)
	}
	if err = WithdrawSource(f.ctx, f.scope, version.Sources[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadReceipt(f.ctx, f.scope, request.ReceiptID); err == nil {
		t.Fatal("receipt remained valid after withdrawal")
	}
}

func TestLearningRejectsLateCheckerAndForgedReceipt(t *testing.T) {
	f := newLearningFixture(t)
	version, intent, checker := f.claim(t)
	late, err := RegisterChecker(f.ctx, f.scope, checker.Spec, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CheckClaim(f.ctx, f.scope, version.ID, late.ID, intent.ID, intent.Generation); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("post-hoc checker accepted", err)
	}
	fake := experience.Activation{IntentID: intent.ID, Generation: intent.Generation, ReceiptID: checker.ID}
	if _, err = f.store.Activate(f.ctx, f.scope, version.ID, fake); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("forged receipt accepted", err)
	}
}
