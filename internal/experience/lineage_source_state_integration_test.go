//go:build integration

package experience

import (
	"context"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestRunLineageDistinguishesSourceInvalidityFromPermissionWithdrawal(t *testing.T) {
	f := lineageFixture(t)
	lineageRecords(t, f)
	for _, test := range []struct {
		err            error
		status, reason string
	}{
		{ErrForbidden, "restricted", "source_unavailable"},
		{ErrInvalidated, "invalid", "source_invalidated"},
		{ErrNotFound, "unproven", "source_missing"},
	} {
		f.store.VerifySource = func(context.Context, pgx.Tx, Scope, Source) error { return test.err }
		got, err := f.store.ReadArtifactSummary(f.ctx, f.scope, "a-parent")
		if err != nil || got.Status != test.status || got.InvalidReason != test.reason || !got.Redacted || got.OutputHash != "" || len(got.References) != 0 {
			t.Fatalf("source cause lost: %+v %v", got, err)
		}
	}
}

func TestRunLineageRedactsMalformedReferencesWithoutSourceProof(t *testing.T) {
	f := lineageFixture(t)
	a := Artifact{ID: "malformed", ProjectID: f.scope.ProjectID, RunID: "actual-run", Hash: "out", InputHash: "logical", ExecutionInputHash: "actual", ProofVersion: 1, References: []Reference{{VersionID: "unverified-private-value", BodyHash: "not-a-hash", DeliveryRevision: 1}}}
	insertAuditFixture(t, f, a)
	got, err := f.store.ReadArtifactSummary(f.ctx, f.scope, a.ID)
	if err != nil || !got.Redacted || len(got.References) != 0 || got.OutputHash != "" {
		t.Fatalf("unverified reference escaped: %+v %v", got, err)
	}
}
