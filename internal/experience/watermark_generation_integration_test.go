//go:build integration

package experience

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestStoreApprovalGenerationSelectsWithinOriginalWatermark(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.serving(t)
	runID := uuid.NewString()
	original, err := f.store.Freeze(f.ctx, f.scope, runID, 1)
	if err != nil {
		t.Fatal(err)
	}
	input := SelectionInput{ProjectID: f.scope.ProjectID, RunID: runID, NodeID: "node", Round: 1, Generation: 2, WatermarkGeneration: 1, ApprovalRevision: 2, BoundInputHash: "bound", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
	usage, versions, err := f.store.Select(f.ctx, f.scope, input)
	if err != nil || len(versions) != 1 || versions[0].ID != version.ID || usage.WatermarkHash != original.Hash {
		t.Fatalf("approval generation changed original watermark: %+v %v", versions, err)
	}
}

func TestStoreExistingAnonymousEmptyWatermarkSurvivesLaterEnablement(t *testing.T) {
	f := newStoreFixture(t)
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	runID := uuid.NewString()
	anonymous := f.scope
	anonymous.ActorID = ""
	tx, err := conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	original, err := (Store{}).FreezeEmptyTx(f.ctx, tx, anonymous, runID, 1, "business")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.policy(t)
	tx, err = conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	retry, err := (Store{}).FreezeEmptyTx(f.ctx, tx, anonymous, runID, 1, "business")
	if err != nil || retry.Hash != original.Hash || len(retry.Versions) != 0 {
		t.Fatalf("old empty snapshot changed: %+v %v", retry, err)
	}
}
