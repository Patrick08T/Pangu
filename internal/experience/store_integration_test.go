//go:build integration

package experience

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type storeFixture struct {
	store Store
	scope Scope
	ctx   context.Context
}

func newStoreFixture(t *testing.T) storeFixture {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("integration requires an independent PANGU_TEST_DATABASE_URL; not skipped")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	scope := Scope{Namespace: uuid.NewString(), ProjectID: uuid.NewString(), ActorID: uuid.NewString()}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs)
	 VALUES($1,$2,'memory-test','unused','main','test','git')`, scope.ProjectID, scope.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	store := Store{Authorize: func(_ context.Context, _ pgx.Tx, got Scope) error {
		if got.Namespace != scope.Namespace || got.ProjectID != scope.ProjectID || got.ActorID != scope.ActorID {
			return ErrForbidden
		}
		return nil
	}, VerifySource: func(_ context.Context, _ pgx.Tx, _ Scope, source Source) error {
		if source.ID != "verified-source" || source.Hash != strings.Repeat("a", 64) {
			return ErrForbidden
		}
		return nil
	}, VerifyIntent: func(_ context.Context, _ pgx.Tx, _ Scope, input IntentInput) error {
		if input.Purpose != "business" || !input.Terminal || !input.StopConfirmed {
			return ErrForbidden
		}
		return nil
	}, VerifyClaim: func(_ context.Context, _ pgx.Tx, _ Scope, _ Version, receipt Activation) error {
		if receipt.ReceiptID != "independent-check" {
			return ErrForbidden
		}
		return nil
	}, VerifyBenefit: func(_ context.Context, _ pgx.Tx, _ Scope, _ Version, change DeliveryChange) error {
		if change.EvidenceID != "independent-benefit" {
			return ErrForbidden
		}
		return nil
	}}
	return storeFixture{store: store, scope: scope, ctx: ctx}
}

func (f storeFixture) create(t *testing.T) Version {
	t.Helper()
	version := sampleVersion(uuid.NewString())
	version.ProjectID, version.ExpiresAt = f.scope.ProjectID, time.Now().Add(time.Hour)
	version.Sources[0].ProjectID, version.Sources[0].ID = f.scope.ProjectID, "verified-source"
	created, err := f.store.Create(f.ctx, f.scope, version)
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func (f storeFixture) policy(t *testing.T) Policy {
	t.Helper()
	policy := enabledPolicy()
	policy.ProjectID, policy.ActorID = f.scope.ProjectID, f.scope.ActorID
	policy.Roles = []string{"builder"}
	policy.Delivery = DeliveryPolicy{Mode: "serving", TaskScope: []string{"approved"}, TrafficBasisPoints: 10000}
	saved, err := f.store.SavePolicy(f.ctx, f.scope, policy, 0)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func (f storeFixture) activate(t *testing.T, version Version) Version {
	t.Helper()
	intent, err := f.store.EnsureLearningIntent(f.ctx, f.scope, IntentInput{SourceRunID: uuid.NewString(),
		EvidenceRevision: 1, PolicyRevision: 1, Purpose: "business", Terminal: true, StopConfirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	active, err := f.store.Activate(f.ctx, f.scope, version.ID, Activation{IntentID: intent.ID, Generation: 1, ReceiptID: "independent-check"})
	if err != nil {
		t.Fatal(err)
	}
	return active
}

func (f storeFixture) serving(t *testing.T) Version {
	t.Helper()
	version := f.activate(t, f.create(t))
	for _, mode := range []string{"canary", "serving"} {
		updated, err := f.store.SetDelivery(f.ctx, f.scope, version.ID, DeliveryChange{ExpectedRevision: version.DeliveryRevision,
			Policy: DeliveryPolicy{Mode: mode, TaskScope: []string{"approved"}, TrafficBasisPoints: 10000}, EvidenceID: "independent-benefit"})
		if err != nil {
			t.Fatal(err)
		}
		version = updated
	}
	return version
}

func TestStorePersistsImmutableMetadataAndRemovableBody(t *testing.T) {
	f := newStoreFixture(t)
	version := f.create(t)
	if version.Validity != "candidate" || version.Delivery != "shadow" {
		t.Fatalf("client promoted itself: %+v", version)
	}
	read, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil || read.BodyHash == "" || read.Content.Practice != version.Content.Practice {
		t.Fatalf("persisted: %+v %v", read, err)
	}
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	if _, err := conn.Exec(f.ctx, `UPDATE pangu.experience_versions SET body_hash=$2 WHERE id=$1`, version.ID, strings.Repeat("b", 64)); err == nil {
		t.Fatal("immutable metadata changed")
	}
	if _, err := conn.Exec(f.ctx, `UPDATE pangu.experience_bodies SET content='{}' WHERE version_id=$1`, version.ID); err == nil {
		t.Fatal("immutable body changed")
	}
	if err := f.store.Revoke(f.ctx, f.scope, version.ID, "source withdrawn"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Read(f.ctx, f.scope, version.ID); !errors.Is(err, ErrInvalidated) {
		t.Fatalf("revoked body readable: %v", err)
	}
	var bodies int
	if err := conn.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_bodies WHERE version_id=$1`, version.ID).Scan(&bodies); err != nil || bodies != 0 {
		t.Fatalf("body retained: %d %v", bodies, err)
	}
}

func TestStoreRechecksSourceAndProjectAuthorization(t *testing.T) {
	f := newStoreFixture(t)
	version := f.create(t)
	f.store.VerifySource = func(context.Context, pgx.Tx, Scope, Source) error { return ErrForbidden }
	if _, err := f.store.Read(f.ctx, f.scope, version.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("source withdrawal ignored: %v", err)
	}
	f.scope.ProjectID = uuid.NewString()
	if _, err := f.store.Read(f.ctx, f.scope, version.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign project: %v", err)
	}
}

func TestStorePolicyUsesRevisionCASAndDefaultsDisabled(t *testing.T) {
	f := newStoreFixture(t)
	initial, err := f.store.Policy(f.ctx, f.scope)
	if err != nil || initial.Enabled || initial.ReadEnabled {
		t.Fatalf("default policy: %+v %v", initial, err)
	}
	policy := f.policy(t)
	if _, err := f.store.SavePolicy(f.ctx, f.scope, policy, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	policy.Enabled = false
	if _, err := f.store.SavePolicy(f.ctx, f.scope, policy, 1); err != nil {
		t.Fatal(err)
	}
}
