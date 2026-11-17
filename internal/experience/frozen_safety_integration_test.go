//go:build integration

package experience

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"testing"
)

func appendFrozenSafety(t *testing.T, f storeFixture, versionID string, blocked bool) {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	_, err = conn.Exec(f.ctx, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,evidence_id,actor_id)
 SELECT version_id,revision+1,'paused',jsonb_set(policy,'{mode}','"paused"')||jsonb_build_object('block_frozen',$2::boolean),'trusted-safety-receipt','server'
 FROM pangu.experience_deliveries WHERE version_id=$1 ORDER BY revision DESC LIMIT 1`, versionID, blocked)
	if err != nil {
		t.Fatal(err)
	}
}

func frozenSafetyInput(f storeFixture) SelectionInput {
	return SelectionInput{ProjectID: f.scope.ProjectID, RunID: "accepted-request", NodeID: "node-a", Round: 1, Generation: 1, ApprovalRevision: 1, BoundInputHash: "logical", RoleID: "builder", CanaryScope: "approved", Text: "lock"}
}

func TestFrozenSafetyGateBlocksExactRetryReferencesAndAncestor(t *testing.T) {
	f := lineageFixture(t)
	v := lineageRecords(t, f)
	appendFrozenSafety(t, f, v.ID, true)
	if _, _, err := f.store.Select(f.ctx, f.scope, frozenSafetyInput(f)); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("confirmed safety failure reused frozen input: %v", err)
	}
	if err := f.store.ValidateReferences(f.ctx, f.scope, []Reference{versionReference(v)}); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("blocked reference accepted: %v", err)
	}
	if err := f.store.CheckArtifactInput(f.ctx, f.scope, ArtifactReference{ID: "b-child", Hash: "output-b"}, "logical-b"); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("unsafe ancestor reused: %v", err)
	}
	read, err := f.store.Read(f.ctx, f.scope, v.ID)
	if err != nil || read.Validity != "active" || read.Content.Practice == "" {
		t.Fatalf("safety gate rewrote claim/body: %+v %v", read, err)
	}
}

func TestFrozenSafetyGateIsStickyAndCannotBeClearedByPublicDelivery(t *testing.T) {
	f := lineageFixture(t)
	v := lineageRecords(t, f)
	appendFrozenSafety(t, f, v.ID, true)
	_, err := f.store.SetDelivery(f.ctx, f.scope, v.ID, DeliveryChange{ExpectedRevision: v.DeliveryRevision + 1, Policy: DeliveryPolicy{Mode: "paused"}})
	if !errors.Is(err, ErrInvalidated) {
		t.Fatalf("public pause cleared machine safety gate: %v", err)
	}
	appendFrozenSafety(t, f, v.ID, false)
	if err = f.store.ValidateReferences(f.ctx, f.scope, []Reference{versionReference(v)}); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("later false erased safety history: %v", err)
	}
	summary, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "a-parent")
	if err != nil || summary.InvalidReason != "frozen_safety_blocked" || summary.Status != "invalid" {
		t.Fatalf("safety cause not auditable: %+v %v", summary, err)
	}
}

func TestFrozenSafetyOrdinaryPausePreservesExactRetry(t *testing.T) {
	f := lineageFixture(t)
	v := lineageRecords(t, f)
	appendFrozenSafety(t, f, v.ID, false)
	usage, versions, err := f.store.Select(f.ctx, f.scope, frozenSafetyInput(f))
	if err != nil || len(versions) != 1 || len(usage.References) != 1 {
		t.Fatalf("ordinary pause invalidated frozen retry: %+v %v", usage, err)
	}
	input := frozenSafetyInput(f)
	input.Round++
	_, versions, err = f.store.Select(f.ctx, f.scope, input)
	if err != nil || len(versions) != 0 {
		t.Fatalf("ordinary pause allowed new selection: %+v %v", versions, err)
	}
}
