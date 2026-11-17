//go:build integration

package experience

import (
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"testing"
)

func insertAuditFixture(t *testing.T, f storeFixture, a Artifact) {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(f.ctx, `INSERT INTO pangu.experience_artifacts(namespace,project_id,id,artifact)VALUES($1,$2,$3,$4)`, f.scope.Namespace, f.scope.ProjectID, a.ID, data); err != nil {
		t.Fatal(err)
	}
}

func TestRunLineageReportsPolicyWithdrawalWithoutHidingAudit(t *testing.T) {
	f := lineageFixture(t)
	lineageRecords(t, f)
	policy, err := f.store.Policy(f.ctx, f.scope)
	if err != nil {
		t.Fatal(err)
	}
	policy.ReadEnabled = false
	if _, err = f.store.SavePolicy(f.ctx, f.scope, policy, policy.Revision); err != nil {
		t.Fatal(err)
	}
	parent, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "a-parent")
	if err != nil || parent.Status != "invalid" || parent.InvalidReason != "policy_read_revoked" || parent.Redacted || len(parent.References) != 1 {
		t.Fatalf("withdrawn read mislabeled: %+v %v", parent, err)
	}
}

func TestRunLineageNeverCallsUnknownAncestorACompleteChain(t *testing.T) {
	f := lineageFixture(t)
	if _, err := f.store.Freeze(f.ctx, f.scope, "accepted-request", 1); err != nil {
		t.Fatal(err)
	}
	a := Artifact{ID: "orphan", ProjectID: f.scope.ProjectID, RunID: "actual-run", Hash: "out", InputHash: "logical", ExecutionInputHash: "actual", ProofVersion: 1, Parents: []ArtifactReference{{ID: "missing-parent", Hash: "claimed"}}}
	insertAuditFixture(t, f, a)
	got, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery())
	if err != nil || got.ChainComplete || len(got.Artifacts.Items) != 1 || got.Artifacts.Items[0].Status != "unproven" || got.Artifacts.Items[0].InvalidReason != "ancestor_unproven" {
		t.Fatalf("unknown ancestor mislabeled: %+v %v", got, err)
	}
	missing, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "missing-parent")
	if err != nil || missing.InvalidReason != "artifact_missing" || missing.Proven {
		t.Fatalf("missing proof: %+v %v", missing, err)
	}
}

func TestRunLineageCycleIsUnprovenAndBounded(t *testing.T) {
	f := lineageFixture(t)
	a := Artifact{ID: "cycle", ProjectID: f.scope.ProjectID, RunID: "actual-run", Hash: "out", InputHash: "logical", ExecutionInputHash: "actual", ProofVersion: 1, Parents: []ArtifactReference{{ID: "cycle", Hash: "out"}}}
	insertAuditFixture(t, f, a)
	got, err := f.store.ReadArtifactSummary(f.ctx, f.scope, a.ID)
	if err != nil || got.Status != "unproven" || got.Proven {
		t.Fatalf("cycle proof accepted: %+v %v", got, err)
	}
}

func TestRunLineageValidatesPagesAndReportsCompleteClosure(t *testing.T) {
	f := lineageFixture(t)
	lineageRecords(t, f)
	query := lineageQuery()
	query.UsagePage.Limit = 0
	if _, err := f.store.ReadRunLineage(f.ctx, f.scope, query); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid page accepted: %v", err)
	}
	query = lineageQuery()
	query.UsagePage.Limit, query.ArtifactPage.Limit = 100, 100
	got, err := f.store.ReadRunLineage(f.ctx, f.scope, query)
	if err != nil || !got.ChainComplete || got.Usages.HasMore || got.Artifacts.HasMore {
		t.Fatalf("full recorded closure not proven complete: %+v %v", got, err)
	}
}
