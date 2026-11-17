//go:build integration

package experience

import (
	"errors"
	"testing"
)

func TestStoreLearningDisablePreservesAuthorizedFrozenBusinessInput(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.serving(t)
	if _, err := f.store.Freeze(f.ctx, f.scope, "old-run", 1); err != nil {
		t.Fatal(err)
	}
	policy, err := f.store.Policy(f.ctx, f.scope)
	if err != nil {
		t.Fatal(err)
	}
	policy.Enabled = false
	if _, err = f.store.SavePolicy(f.ctx, f.scope, policy, 1); err != nil {
		t.Fatal(err)
	}
	input := SelectionInput{ProjectID: f.scope.ProjectID, RunID: "old-run", NodeID: "node", Generation: 1, Round: 1, ApprovalRevision: 1, BoundInputHash: "bound", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	_, versions, err := f.store.Select(f.ctx, f.scope, input)
	if err != nil || len(versions) != 1 || versions[0].ID != version.ID {
		t.Fatalf("learning disable rewrote frozen input: %+v %v", versions, err)
	}
	if err = f.store.ValidateReferences(f.ctx, f.scope, []Reference{versionReference(version)}); err != nil {
		t.Fatalf("learning disable invalidated readable proof: %v", err)
	}
	fresh, err := f.store.Freeze(f.ctx, f.scope, "new-run", 1)
	if err != nil || len(fresh.Versions) != 0 {
		t.Fatalf("learning disable admitted new memory freeze: %+v %v", fresh, err)
	}
}

func TestStorePolicyWithdrawalInvalidatesExistingArtifactReuse(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.serving(t)
	artifact := Artifact{ID: "policy-proof", ProjectID: f.scope.ProjectID, Kind: "output", Hash: "output-hash", InputHash: "effective-input", ProofVersion: 1, References: []Reference{versionReference(version)}}
	if err := f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
		t.Fatal(err)
	}
	policy, err := f.store.Policy(f.ctx, f.scope)
	if err != nil {
		t.Fatal(err)
	}
	policy.Enabled, policy.ReadEnabled, policy.Revoked = false, false, true
	if _, err = f.store.SavePolicy(f.ctx, f.scope, policy, 1); err != nil {
		t.Fatal(err)
	}
	if err = f.store.CheckArtifact(f.ctx, f.scope, ArtifactReference{ID: artifact.ID, Hash: artifact.Hash}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("withdrawn policy left reusable artifact: %v", err)
	}
}
