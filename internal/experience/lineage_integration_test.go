//go:build integration

package experience

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
)

func lineageFixture(t *testing.T) storeFixture {
	t.Helper()
	f := newStoreFixture(t)
	f.store.VerifyRun = func(ctx context.Context, tx pgx.Tx, scope Scope, query RunLineageQuery) (RunBinding, error) {
		var one int
		if err := tx.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
			return RunBinding{}, err
		}
		if query.ActualRunID != "actual-run" {
			return RunBinding{}, ErrForbidden
		}
		return RunBinding{Namespace: scope.Namespace, ProjectID: scope.ProjectID, ActualRunID: "actual-run", RequestID: "accepted-request", WatermarkGeneration: 1}, nil
	}
	return f
}

func lineageQuery() RunLineageQuery {
	return RunLineageQuery{ActualRunID: "actual-run", UsagePage: Page{Limit: 1}, ArtifactPage: Page{Limit: 1}}
}

func lineageRecords(t *testing.T, f storeFixture) Version {
	t.Helper()
	f.policy(t)
	v := f.serving(t)
	if _, err := f.store.Freeze(f.ctx, f.scope, "accepted-request", 1); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"node-a", "node-b"} {
		input := SelectionInput{ProjectID: f.scope.ProjectID, RunID: "accepted-request", NodeID: node, Round: 1, Generation: 1, ApprovalRevision: 1, BoundInputHash: "logical", RoleID: "builder", CanaryScope: "approved", Text: "lock"}
		if _, _, err := f.store.Select(f.ctx, f.scope, input); err != nil {
			t.Fatal(err)
		}
	}
	parent := Artifact{ID: "a-parent", RunID: "actual-run", NodeID: "node-a", Kind: "output", ProjectID: f.scope.ProjectID, Hash: "output-a", InputHash: "logical-a", ExecutionInputHash: "actual-a", Generation: 1, ProofVersion: 1, References: []Reference{versionReference(v)}}
	child := Artifact{ID: "b-child", RunID: "actual-run", NodeID: "node-b", Kind: "checkpoint", ProjectID: f.scope.ProjectID, Hash: "output-b", InputHash: "logical-b", ExecutionInputHash: "actual-b", Generation: 1, ProofVersion: 1, Parents: []ArtifactReference{{ID: parent.ID, Hash: parent.Hash}}}
	for _, proof := range []Artifact{parent, child} {
		if err := f.store.SaveArtifact(f.ctx, f.scope, proof); err != nil {
			t.Fatal(err)
		}
	}
	return v
}

func TestRunLineageUsesVerifiedBindingAndReportsPagination(t *testing.T) {
	f := lineageFixture(t)
	lineageRecords(t, f)
	query := lineageQuery()
	got, err := f.store.ReadRunLineage(f.ctx, f.scope, query)
	if err != nil || got.ActualRunID != "actual-run" || !got.Watermark.Present || got.Watermark.Status != "valid" || len(got.Watermark.References) != 1 {
		t.Fatalf("accepted lineage missing: %+v %v", got, err)
	}
	if got.Usages.Total != 2 || !got.Usages.HasMore || len(got.Usages.Items) != 1 || got.Usages.Items[0].MatchReason != "scoped_error_or_keyword_match" || got.Usages.Items[0].InjectionHash == "" {
		t.Fatalf("usage page: %+v", got.Usages)
	}
	if got.Artifacts.Total != 2 || !got.Artifacts.HasMore || got.Artifacts.Items[0].ID != "a-parent" || got.ChainComplete {
		t.Fatalf("incomplete chain presented complete: %+v", got)
	}
	query.UsagePage.AfterID, query.ArtifactPage.AfterID = got.Usages.NextCursor, got.Artifacts.NextCursor
	next, err := f.store.ReadRunLineage(f.ctx, f.scope, query)
	if err != nil || next.Usages.HasMore || next.Artifacts.HasMore || next.Artifacts.Items[0].ID != "b-child" || next.Usages.Items[0].NodeID == got.Usages.Items[0].NodeID {
		t.Fatalf("next page: %+v %v", next, err)
	}
}

func TestRunLineageDistinguishesMissingFromFrozenEmptyWatermark(t *testing.T) {
	f := lineageFixture(t)
	got, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery())
	if err != nil || got.Watermark.Present || got.Watermark.Proven || got.Watermark.InvalidReason != "watermark_missing" {
		t.Fatalf("legacy absence: %+v %v", got, err)
	}
	if _, err = f.store.Freeze(f.ctx, f.scope, "accepted-request", 1); err != nil {
		t.Fatal(err)
	}
	got, err = f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery())
	if err != nil || !got.Watermark.Present || !got.Watermark.Proven || len(got.Watermark.References) != 0 {
		t.Fatalf("frozen empty: %+v %v", got, err)
	}
}

func TestRunLineageRejectsUnverifiedAndCrossProjectAliases(t *testing.T) {
	f := lineageFixture(t)
	original := f.store.VerifyRun
	f.store.VerifyRun = nil
	if _, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unverified run admitted: %v", err)
	}
	f.store.VerifyRun = original
	query := lineageQuery()
	query.RequestID = "foreign-request"
	if _, err := f.store.ReadRunLineage(f.ctx, f.scope, query); !errors.Is(err, ErrForbidden) {
		t.Fatalf("request alias accepted: %v", err)
	}
	f.store.VerifyRun = func(ctx context.Context, tx pgx.Tx, scope Scope, q RunLineageQuery) (RunBinding, error) {
		b, e := original(ctx, tx, scope, q)
		b.ProjectID = "foreign-project"
		return b, e
	}
	if _, err := f.store.ReadRunLineage(f.ctx, f.scope, lineageQuery()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-project binding accepted: %v", err)
	}
}

func TestRunLineageKeepsRevokedInfluenceAuditable(t *testing.T) {
	f := lineageFixture(t)
	v := lineageRecords(t, f)
	if err := f.store.Revoke(f.ctx, f.scope, v.ID, "private free text must never escape"); err != nil {
		t.Fatal(err)
	}
	root, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "b-child")
	if err != nil || root.Status != "invalid" || root.InvalidReason != "ancestor_invalid" || len(root.Parents) != 1 {
		t.Fatalf("revoked ancestor disappeared: %+v %v", root, err)
	}
	parent, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "a-parent")
	if err != nil || parent.InvalidReason != "experience_revoked" || len(parent.References) != 1 {
		t.Fatalf("revoked direct evidence disappeared: %+v %v", parent, err)
	}
	data, _ := json.Marshal(parent)
	if strings.Contains(string(data), "private free text") {
		t.Fatal("private invalidation reason leaked")
	}
}
