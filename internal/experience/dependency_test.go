package experience

import (
	"errors"
	"testing"
)

func TestArtifactValidationTraversesUpstreamAndFailsClosed(t *testing.T) {
	upstream := Artifact{ID: "upstream", Hash: "output-a", InputHash: "input-a", ProofVersion: 1,
		References: []Reference{{VersionID: "revoked"}}}
	child := Artifact{ID: "child", Hash: "output-b", InputHash: "input-b", ProofVersion: 1,
		Parents: []ArtifactReference{{ID: "upstream", Hash: "output-a"}}}
	graph := map[string]Artifact{"child": child, "upstream": upstream}
	lookup := func(id string) (Artifact, error) {
		artifact, exists := graph[id]
		if !exists {
			return Artifact{}, ErrNotFound
		}
		return artifact, nil
	}
	check := func(refs []Reference) error {
		if len(refs) > 0 {
			return ErrInvalidated
		}
		return nil
	}
	if err := ValidateArtifact(ArtifactReference{ID: "child", Hash: "output-b"}, lookup, check); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("transitive revocation bypassed: %v", err)
	}
	upstream.References = nil
	graph["upstream"] = upstream
	if err := ValidateArtifact(ArtifactReference{ID: "child", Hash: "output-b"}, lookup, check); err != nil {
		t.Fatal(err)
	}
	delete(graph, "upstream")
	if err := ValidateArtifact(ArtifactReference{ID: "child", Hash: "output-b"}, lookup, check); err == nil {
		t.Fatal("missing proof treated as no dependency")
	}
}

func TestArtifactValidationRejectsCyclesHashesAndLegacyProofs(t *testing.T) {
	artifact := Artifact{ID: "a", Hash: "a-hash", InputHash: "input", ProofVersion: 1}
	lookup := func(string) (Artifact, error) { return artifact, nil }
	check := func([]Reference) error { return nil }
	if ValidateArtifact(ArtifactReference{ID: "a", Hash: "changed"}, lookup, check) == nil {
		t.Fatal("changed output reused")
	}
	artifact.ProofVersion = 0
	if ValidateArtifact(ArtifactReference{ID: "a", Hash: "a-hash"}, lookup, check) == nil {
		t.Fatal("legacy output reused")
	}
	artifact.ProofVersion = 1
	artifact.Parents = []ArtifactReference{{ID: "a", Hash: "a-hash"}}
	if ValidateArtifact(ArtifactReference{ID: "a", Hash: "a-hash"}, lookup, check) == nil {
		t.Fatal("cyclic proof accepted")
	}
}

func TestLearningIntentRequiresBusinessTerminationAndFreshGeneration(t *testing.T) {
	input := IntentInput{SourceRunID: "run", EvidenceRevision: 1, PolicyRevision: 1,
		Purpose: "business", Terminal: true, StopConfirmed: true}
	intent, err := NewLearningIntent(input)
	if err != nil || intent.Generation != 1 {
		t.Fatalf("new intent: %+v %v", intent, err)
	}
	for _, purpose := range []string{"learning", "evaluation", "candidate", ""} {
		input.Purpose = purpose
		if _, err := NewLearningIntent(input); err == nil {
			t.Fatal("recursive learning accepted")
		}
	}
	input.Purpose, input.StopConfirmed = "business", false
	if _, err := NewLearningIntent(input); err == nil {
		t.Fatal("unconfirmed execution learned")
	}
	if _, err := ApplyIntentReceipt(intent, 0, "old", "completed"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale receipt accepted")
	}
	finished, err := ApplyIntentReceipt(intent, 1, "receipt", "completed")
	if err != nil || finished.Stage != "completed" {
		t.Fatalf("receipt: %+v %v", finished, err)
	}
	if _, err := ApplyIntentReceipt(finished, 1, "different", "completed"); !errors.Is(err, ErrConflict) {
		t.Fatal("receipt overwritten")
	}
}
