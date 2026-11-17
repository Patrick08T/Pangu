//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func evolutionNextFlowFixture(t *testing.T, f evolutionFixture, task string) *catalog.Version {
	t.Helper()
	ctx := context.Background()
	v, err := catalog.ReadVersion(ctx, f.scope.Namespace, f.request.Payload.FlowVersion)
	if err != nil {
		t.Fatal(err)
	}
	if task != "" {
		v.Spec.Flow.Nodes[0].Task = task
	}
	draft, err := catalog.Save(ctx, f.scope.Namespace, v.ObjectID, v.SourceRevision, "full flow", v.Spec)
	if err != nil {
		t.Fatal(err)
	}
	next, err := catalog.Publish(ctx, f.scope.Namespace, v.ObjectID, draft.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestEvolutionEntryCASPreservesUserEditsAndPersistsRemoteIntent(t *testing.T) {
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	old := f.request.Payload.FlowVersion
	next := evolutionNextFlowFixture(t, f, "new tested task")
	d, err := catalog.Compile(ctx, f.scope.Namespace, old, f.scope.ProjectID, "business input")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID, hookID := uuid.NewString(), uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.schedules(id,namespace,project_id,name,expression,workflow,created_by,remote_synced) VALUES($1,$2,$3,$6,'0 * * * *',$4,$5,true)`, scheduleID, f.scope.Namespace, f.scope.ProjectID, evolutionJSON(d), f.scope.ActorID, scheduleID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.github_hooks(id,namespace,project_id,actor_id,version_id,repository,label,secret_ref) VALUES($1,$2,$3,$4,$5,'test/repo','agent','PANGU_HOOK_SECRET_TEST')`, hookID, f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, old)
	if err != nil {
		t.Fatal(err)
	}
	adapter := EvolutionEntries{}
	for _, target := range []evolution.EntryTarget{{Kind: "cron", ID: scheduleID}, {Kind: "webhook", ID: hookID}} {
		var before, after evolution.EntrySnapshot
		err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
			var e error
			before, e = adapter.ReadTx(ctx, tx, f.scope, target)
			if e != nil {
				return e
			}
			after, e = adapter.CompareAndSwapTx(ctx, tx, f.scope, "effect:"+target.Key(), before, next.ID)
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if after.VersionID != next.ID || after.Revision <= before.Revision {
			t.Fatalf("CAS did not change future binding: %+v", after)
		}
		err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
			_, e := adapter.CompareAndSwapTx(ctx, tx, f.scope, "stale:"+target.Key(), before, next.ID)
			return e
		})
		if !errors.Is(err, evolution.ErrConflict) {
			t.Fatalf("stale revision accepted: %v", err)
		}
	}
	var outbox int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.evolution_remote_intents WHERE namespace=$1 AND status='pending'`, f.scope.Namespace).Scan(&outbox); err != nil || outbox != 1 {
		t.Fatal(outbox, err)
	}
	_, err = conn.Exec(ctx, `UPDATE pangu.schedules SET paused=true WHERE id=$1`, scheduleID)
	if err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		current, e := adapter.ReadTx(ctx, tx, f.scope, evolution.EntryTarget{Kind: "cron", ID: scheduleID})
		if e != nil {
			return e
		}
		_, e = adapter.CompareAndSwapTx(ctx, tx, f.scope, "paused", current, old)
		return e
	})
	if !errors.Is(err, evolution.ErrConflict) {
		t.Fatalf("paused manual entry was re-enabled: %v", err)
	}
}
