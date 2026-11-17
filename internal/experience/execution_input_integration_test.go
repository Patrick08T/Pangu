//go:build integration

package experience

import (
	"encoding/json"
	"testing"
)

func TestStoreArtifactPreservesLogicalAndActualExecutionInputHashes(t *testing.T) {
	f := newStoreFixture(t)
	var artifact Artifact
	data := []byte(`{"id":"actual-input-proof","kind":"output","hash":"output","input_hash":"logical-dispatch","execution_input_hash":"resolved-worker-input","proof_version":1}`)
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	artifact.ProjectID = f.scope.ProjectID
	if err := f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
		t.Fatal(err)
	}
	stored, err := f.store.ReadArtifact(f.ctx, f.scope, artifact.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["execution_input_hash"] != "resolved-worker-input" || fields["input_hash"] != "logical-dispatch" {
		t.Fatalf("actual/logical input evidence lost: %s", data)
	}
	if err = f.store.CheckArtifactInput(f.ctx, f.scope, ArtifactReference{ID: artifact.ID, Hash: artifact.Hash}, "logical-dispatch"); err != nil {
		t.Fatalf("logical dispatch comparison changed: %v", err)
	}
}
