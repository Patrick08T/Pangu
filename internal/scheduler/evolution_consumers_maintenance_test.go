//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestEvolutionConsumersMaintenanceCancelsThenClears(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	record := f.submit(t, f.target.VersionIDs[0], true, false)
	f.expire(t)
	calls := 0
	cancel := func(ctx context.Context, s evolution.Scope, c evolutionConsumer) error {
		calls++
		requested, err := execution.CancelRequested(ctx, c.run)
		if err != nil || !requested {
			t.Fatalf("remote action preceded durable scoped cancellation %v %v", requested, err)
		}
		return execution.Reconciled(ctx, c.run)
	}
	out, err := reconcileEvolutionRetention(context.Background(), f.serverScope(), f.target.IntentID, 1, cancel)
	if err != nil || out.Deferred || !out.Due || out.VersionsCleared != 1 || calls != 1 {
		t.Fatalf("real retention helper failed %+v %v calls=%d run=%s", out, err, calls, record.RunID)
	}
}

func TestEvolutionConsumersMaintenancePreservesRevocationOnRPCFailure(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	f.submit(t, f.target.VersionIDs[0], false, false)
	f.expire(t)
	failure := errors.New("remote status unavailable")
	out, err := reconcileEvolutionRetention(context.Background(), f.serverScope(), f.target.IntentID, 1, func(context.Context, evolution.Scope, evolutionConsumer) error { return failure })
	if !errors.Is(err, failure) || !out.Due || !out.Deferred {
		t.Fatalf("unknown stop lost due/deferred state %+v %v", out, err)
	}
	f.assertBody(t, true)
}

func TestEvolutionConsumersMaintenancePolicyDisableDoesNotCancel(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	f.submit(t, f.target.VersionIDs[0], false, false)
	f.exec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT namespace,project_id,revision+1,actor_id,jsonb_set(jsonb_set(policy,'{enabled}','false'),'{revision}',to_jsonb(revision+1)) FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=1`, f.scope.Namespace, f.scope.ProjectID)
	out, err := reconcileEvolutionRetention(context.Background(), f.serverScope(), f.target.IntentID, 1, func(context.Context, evolution.Scope, evolutionConsumer) error {
		t.Fatal("ordinary policy disable cancelled business")
		return nil
	})
	if err != nil || out.Due {
		t.Fatalf("disable became retention expiry %+v %v", out, err)
	}
	f.assertBody(t, true)
}

func TestEvolutionConsumersCancellationRejectsForeignContinuation(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	root := f.submit(t, f.target.VersionIDs[0], true, false)
	foreign := f.sameNamespaceProject(t)
	child := foreign.submit(t, foreign.target.VersionIDs[0], true, false)
	f.link(t, root, child)
	err := evolutionConsumerRequestCancel(context.Background(), f.scope, evolutionConsumer{request: root.ID, run: root.RunID})
	if !errors.Is(err, evolution.ErrConflict) {
		t.Fatalf("foreign closure not refused %v", err)
	}
	for _, run := range []string{root.RunID, child.RunID} {
		requested, err := execution.CancelRequested(context.Background(), run)
		if err != nil || requested {
			t.Fatalf("partial/foreign cancellation %s %v %v", run, requested, err)
		}
	}
}

func TestEvolutionConsumersContinuationStopIsRequired(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	root := f.submit(t, f.target.VersionIDs[0], true, false)
	child := f.submit(t, f.request.BaselineVersion, false, false)
	f.link(t, root, child)
	f.stop(t, root.RunID)
	if ok, err := f.guard(t, "stopped"); ok || !errors.Is(err, ErrEvolutionConsumerStopUnknown) {
		t.Fatalf("continuation omitted %v %v", ok, err)
	}
	f.stop(t, child.RunID)
	if ok, err := f.guard(t, "stopped"); err != nil || !ok {
		t.Fatal(ok, err)
	}
}

func TestEvolutionConsumersMaintenanceCallbackIsNotStopProof(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	f.submit(t, f.target.VersionIDs[0], true, false)
	f.expire(t)
	out, err := reconcileEvolutionRetention(context.Background(), f.serverScope(), f.target.IntentID, 1, func(context.Context, evolution.Scope, evolutionConsumer) error { return nil })
	if !errors.Is(err, ErrEvolutionConsumerStopUnknown) || !out.Deferred || !out.Due {
		t.Fatalf("callback success fabricated stop proof %+v %v", out, err)
	}
	f.assertBody(t, true)
}

func TestEvolutionConsumersMaintenancePendingIsDeferred(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	f.submit(t, f.target.VersionIDs[0], true, true)
	f.expire(t)
	out, err := reconcileEvolutionRetention(context.Background(), f.serverScope(), f.target.IntentID, 1, func(context.Context, evolution.Scope, evolutionConsumer) error {
		t.Fatal("pending unknown run caused RPC")
		return nil
	})
	if !errors.Is(err, ErrEvolutionConsumerStopUnknown) || !out.Deferred || !out.Due {
		t.Fatalf("pending consumer cleared %+v %v", out, err)
	}
	f.assertBody(t, true)
}

func TestEvolutionConsumersMaintenanceVisitsEveryConsumerPage(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	for i := 0; i < 65; i++ {
		f.submit(t, f.target.VersionIDs[0], i%2 == 0, false)
	}
	f.expire(t)
	calls := 0
	out, err := reconcileEvolutionRetention(context.Background(), f.serverScope(), f.target.IntentID, 1, func(ctx context.Context, _ evolution.Scope, c evolutionConsumer) error {
		calls++
		return execution.Reconciled(ctx, c.run)
	})
	if err != nil || out.Deferred || out.VersionsCleared != 1 || calls != 65 {
		t.Fatalf("consumer pagination skipped a live run %+v %v calls=%d", out, err, calls)
	}
}

func (f evolutionConsumersFixture) expire(t *testing.T) {
	t.Helper()
	f.exec(t, `UPDATE pangu.evolution_intents SET created_at=clock_timestamp()-interval '2 days',state='stable' WHERE id=$1`, f.target.IntentID)
}

func (f evolutionConsumersFixture) serverScope() evolution.Scope {
	return evolution.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID}
}

func (f evolutionConsumersFixture) assertBody(t *testing.T, present bool) {
	t.Helper()
	var found bool
	err := evolutionTransaction(context.Background(), func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT b.spec IS NOT NULL AND s.state='revoked' FROM pangu.catalog_version_bodies b JOIN pangu.catalog_version_state s ON s.version_id=b.version_id WHERE b.version_id=$1`, f.target.VersionIDs[0]).Scan(&found)
	})
	if err != nil || found != present {
		t.Fatalf("body/revocation differs %v %v", found, err)
	}
}

func (f evolutionConsumersFixture) link(t *testing.T, root, child *submission.Record) {
	t.Helper()
	approval := uuid.NewString()
	f.exec(t, `INSERT INTO pangu.approvals(id,namespace,project_id,request_id,run_id,node_id,input_hash,subject_hash,head_commit) VALUES($1,$2,$3,$4,$5,'gate','input','subject','head')`, approval, f.scope.Namespace, f.scope.ProjectID, root.ID, root.RunID)
	f.exec(t, `INSERT INTO pangu.plan_continuations(approval_id,request_id,version_id) VALUES($1,$2,$3)`, approval, child.ID, f.request.BaselineVersion)
}
