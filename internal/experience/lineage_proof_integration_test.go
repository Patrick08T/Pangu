//go:build integration

package experience

import "testing"

func TestRunLineageDoesNotInferCompleteProofFromZeroArtifacts(t *testing.T) {
	f := lineageFixture(t)
	if _, err := f.store.Freeze(f.ctx, f.scope, "accepted-request", 1); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery())
	if err != nil || got.ChainComplete || got.Artifacts.Total != 0 {
		t.Fatalf("absence of runtime proof called complete: %+v %v", got, err)
	}
}
