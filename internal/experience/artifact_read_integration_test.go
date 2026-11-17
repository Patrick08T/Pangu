//go:build integration

package experience

import (
	"errors"
	"testing"
)

func TestStoreReadsArtifactAndComparesEffectiveInputBeforeReuse(t *testing.T) {
	f := newStoreFixture(t)
	artifact := Artifact{ID: "node-proof", ProjectID: f.scope.ProjectID, Kind: "output", Hash: "output-hash", InputHash: "effective-input", ProofVersion: 1}
	if err := f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.ReadArtifact(f.ctx, f.scope, artifact.ID)
	if err != nil || got.ID != artifact.ID || got.InputHash != "effective-input" {
		t.Fatalf("proof read: %+v %v", got, err)
	}
	ref := ArtifactReference{ID: artifact.ID, Hash: artifact.Hash}
	if err = f.store.CheckArtifactInput(f.ctx, f.scope, ref, "different-input"); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("stale effective input reused: %v", err)
	}
	if err = f.store.CheckArtifactInput(f.ctx, f.scope, ref, "effective-input"); err != nil {
		t.Fatal(err)
	}
}
