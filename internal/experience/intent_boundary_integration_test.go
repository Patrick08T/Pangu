//go:build integration

package experience

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestStoreRejectsIntentWithoutAuthoritativeTerminalVerifier(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	f.store.VerifyIntent = nil
	_, err := f.store.EnsureLearningIntent(f.ctx, f.scope, IntentInput{SourceRunID: uuid.NewString(), EvidenceRevision: 1, PolicyRevision: 1, Purpose: "business", Terminal: true, StopConfirmed: true})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("client terminal flags trusted: %v", err)
	}
}

func TestStoreRejectsChangedEffectiveInputEvenForValidArtifact(t *testing.T) {
	f := newStoreFixture(t)
	artifact := Artifact{ID: "input-proof", ProjectID: f.scope.ProjectID, Kind: "output", Hash: "output-hash", InputHash: "old-input", ProofVersion: 1}
	if err := f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
		t.Fatal(err)
	}
	if err := f.store.CheckArtifactInput(f.ctx, f.scope, ArtifactReference{ID: artifact.ID, Hash: artifact.Hash}, "new-input"); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("changed input accepted: %v", err)
	}
}
