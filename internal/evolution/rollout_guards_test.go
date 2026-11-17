//go:build integration

package evolution

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"sync"
	"testing"
)

func TestRolloutObservationHashMustBeHexAndBaselineIndependent(t *testing.T) {
	f := newRolloutFixture(t)
	f.observing(t)
	ctx := context.Background()
	one, two := f.sample(), f.sample()
	two.BaselineRunID = one.BaselineRunID
	reader := rolloutEvidence{map[string]BusinessEvidence{one.RunID: one, two.RunID: two}}
	f.service.Evidence = reader
	if _, err := f.service.Observe(ctx, f.scope, f.id, 1, one.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Observe(ctx, f.scope, f.id, 1, two.RunID); !errors.Is(err, ErrConflict) {
		t.Fatal("baseline reused as independent sample", err)
	}
	invalid := f.sample()
	invalid.SourceHash = strings.Repeat("z", 64)
	reader.samples[invalid.RunID] = invalid
	if _, err := f.service.Observe(ctx, f.scope, f.id, 1, invalid.RunID); !errors.Is(err, ErrChecker) {
		t.Fatal("invalid hash accepted", err)
	}
}

func TestRolloutReplayStableAndWindowGate(t *testing.T) {
	f := newRolloutFixture(t)
	f.observing(t)
	ctx := context.Background()
	reader := rolloutEvidence{samples: map[string]BusinessEvidence{}}
	f.service.Evidence = reader
	for range 2 {
		sample := f.sample()
		reader.samples[sample.RunID] = sample
		if _, err := f.service.Observe(ctx, f.scope, f.id, 1, sample.RunID); err != nil {
			t.Fatal(err)
		}
	}
	rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp() WHERE id=$1`, f.id)
	// 只验证尚未经过窗口，生产时间由DB赋值且不接受客户端覆盖。
	intent, err := f.service.Advance(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "observing" {
		t.Fatal("window bypassed", intent, err)
	}
	rolloutExec(t, `UPDATE pangu.evolution_intents SET observing_at=clock_timestamp()-interval '2 seconds' WHERE id=$1`, f.id)
	intent, err = f.service.Advance(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "stable" {
		t.Fatal(intent, err)
	}
	intent, err = f.service.Advance(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "stable" {
		t.Fatal("committed stable replay failed", intent, err)
	}
}

func TestRolloutParallelPrepareHasOneBindingTransition(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.service.PrepareSwitch(ctx, f.scope, f.id, 1, "workbench:main", "adopt")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	b, claims := f.binding(t)
	if b.Revision != 2 || claims != 2 {
		t.Fatal("concurrent retry advanced twice", b, claims)
	}
}

func TestRolloutChecksEveryPublishedDependency(t *testing.T) {
	f := newRolloutFixture(t)
	// 模拟流程记录仍可读、关联角色版本已经不可用；不能只核验入口指向的版本。
	rolloutExec(t, `UPDATE pangu.evolution_intents SET publication=$2 WHERE id=$1`, f.id, jsonBytes(PublishedVersions{AgentID: uuid.NewString(), FlowID: f.candidate}))
	if _, err := f.service.PrepareSwitch(context.Background(), f.scope, f.id, 1, "workbench:main", "adopt"); !errors.Is(err, ErrConflict) {
		t.Fatal("unavailable role dependency adopted", err)
	}
	b, _ := f.binding(t)
	if b.Revision != 1 {
		t.Fatal("invalid publication changed binding", b)
	}
}

func TestRolloutPrepublicationDoesNotEnterEntryRecovery(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	rolloutExec(t, `UPDATE pangu.evolution_intents SET publication=NULL,state='reconciling' WHERE id=$1`, f.id)
	if _, err := f.service.RequestRollback(ctx, f.scope, f.id, 1, "wrong phase"); !errors.Is(err, ErrConflict) {
		t.Fatal("prepublication accepted by rollout", err)
	}
	if _, err := f.service.Reconcile(ctx, f.scope, f.id, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("entry recovery released evaluation claims", err)
	}
	_, claims := f.binding(t)
	if claims != 2 {
		t.Fatal("evaluation claims released", claims)
	}
}

func TestRolloutInvalidatedMemoryTriggersBoundedCompensation(t *testing.T) {
	f := newRolloutFixture(t)
	f.observing(t)
	ctx := context.Background()
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, err := readIntentTx(ctx, tx, f.scope, f.id)
		if err != nil {
			return err
		}
		intent.Config.Request.MemoryVersionIDs = []string{uuid.NewString()}
		_, err = tx.Exec(ctx, `UPDATE pangu.evolution_intents SET request=$2 WHERE id=$1`, f.id, jsonBytes(intent.Config))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f.service.Memory = experience.Store{Authorize: func(context.Context, pgx.Tx, experience.Scope) error { return experience.ErrInvalidated }}
	intent, err := f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "rolled_back" {
		t.Fatal("invalidated influence retained", intent, err)
	}
	b, claims := f.binding(t)
	if b.AdoptedVersion != f.request.BaselineVersion || claims != 0 {
		t.Fatal(b, claims)
	}
}

func TestRolloutVersionAuthorizationHoldsUntilCommit(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	err := transaction(ctx, func(tx pgx.Tx) error {
		if err := rolloutUsableVersion(ctx, tx, f.scope, f.candidate, true); err != nil {
			return err
		}
		err := transaction(ctx, func(other pgx.Tx) error {
			if _, err := other.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); err != nil {
				return err
			}
			_, err := other.Exec(ctx, `UPDATE pangu.catalog_version_state SET state='revoked',revision=revision+1 WHERE version_id=$1`, f.candidate)
			return err
		})
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
			t.Fatalf("version revoked between authorization and CAS: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRolloutRemovedManagerCannotAdoptOrAuthorizeRollback(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	f.observing(t)
	rolloutExec(t, `UPDATE pangu.users SET is_admin=false WHERE id=$1`, f.scope.ActorID)
	if _, err := f.service.Advance(ctx, f.scope, f.id, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("removed manager advanced", err)
	}
	if _, err := f.service.RequestRollback(ctx, f.scope, f.id, 1, "unauthorized"); !errors.Is(err, ErrForbidden) {
		t.Fatal("removed manager initiated control", err)
	}
	background := f.scope
	background.ActorID = ""
	intent, err := f.service.Reconcile(ctx, background, f.id, 1)
	if err != nil || intent.State != "rolled_back" {
		t.Fatal("bounded recovery failed", intent, err)
	}
}
