//go:build integration

package evolution

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestRetentionScanPaginatesAllStatesAndSkipsOnlyVerifiedErasure(t *testing.T) {
	f := newRetentionFixture(t, "withdrawn")
	ids := []string{f.id}
	for _, state := range []string{"preparing", "evaluating", "passed", "publishing", "switching", "stabilizing", "rolling_back", "reconciling", "stable", "rolled_back", "rejected", "invalidated", "insufficient"} {
		id := uuid.NewString()
		ids = append(ids, id)
		f.exec(t, `INSERT INTO pangu.evolution_intents(id,namespace,project_id,policy_revision,state,request_hash,request,targets,experiment_id,experiment_hash,publication) SELECT $2,namespace,project_id,policy_revision,$3,request_hash,request,targets,experiment_id,experiment_hash,publication FROM pangu.evolution_intents WHERE id=$1`, f.id, id, state)
	}
	// A marker alone is not evidence that retained private bytes were erased.
	f.exec(t, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{_retention}','{"erased_at":"2026-01-01T00:00:00Z"}'::jsonb) WHERE id=$1`, ids[1])
	markedID := ids[1]
	f.exec(t, `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{request,payload,agent}','{}'::jsonb) WHERE id=$1 OR id=$2`, ids[1], ids[2])
	f.exec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.scope.ActorID)
	f.policy.Revision++
	f.policy.Enabled = false
	f.savePolicy(t)
	got := retentionScanIDs(t, f, 2)
	slices.Sort(ids)
	if !slices.Equal(got, ids) {
		t.Fatal("history/disabled original actor dropped or pagination duplicated", got, ids)
	}
	if _, err := f.reconcile(t, retentionAllowed); err != nil {
		t.Fatal(err)
	}
	want := slices.DeleteFunc(ids, func(id string) bool { return id == f.id || id == markedID })
	got = retentionScanIDs(t, f, 1)
	if !slices.Equal(got, want) {
		t.Fatal("erasure marker trusted without body verification", got, want)
	}
}

func retentionScanIDs(t *testing.T, f retentionFixture, limit int) []string {
	t.Helper()
	var ids []string
	cursor := ""
	for page := 0; page < 100; page++ {
		rows, err := ListRetentionTargets(context.Background(), f.scope.Namespace, cursor, limit)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			return ids
		}
		if len(rows) > limit {
			t.Fatal("scan ignored bounded page", len(rows), limit)
		}
		for _, row := range rows {
			if row.Scope.Namespace != f.scope.Namespace || row.Scope.ProjectID != f.scope.ProjectID || row.Scope.ActorID != "" || row.FrozenActorID != f.scope.ActorID || row.Generation != 1 {
				t.Fatal("foreign actor/scope in maintenance target", row)
			}
			ids = append(ids, row.ID)
			cursor = row.ID
		}
	}
	t.Fatal("keyset scan did not terminate")
	return nil
}

func TestRetentionScanRejectsInvalidCursorAndUnknownScopeData(t *testing.T) {
	f := newRetentionFixture(t, "healthy")
	for _, input := range []struct {
		namespace, after string
		limit            int
	}{{"", "", 1}, {f.scope.Namespace, "bad", 1}, {f.scope.Namespace, "", 0}, {f.scope.Namespace, "", 1001}} {
		if _, err := ListRetentionTargets(context.Background(), input.namespace, input.after, input.limit); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid scan bounds accepted", input, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ListRetentionTargets(ctx, f.scope.Namespace, "", 1); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled scan became empty success", err)
	}
	rows, err := ListRetentionTargets(context.Background(), uuid.NewString(), "", 1)
	if err != nil || len(rows) != 0 {
		t.Fatal("foreign namespace leaked work", rows, err)
	}
	f.exec(t, `UPDATE pangu.evolution_intents SET policy_revision=99999 WHERE id=$1`, f.id)
	rows, err = ListRetentionTargets(context.Background(), f.scope.Namespace, "", 1)
	if err == nil || len(rows) != 0 {
		t.Fatal("missing frozen policy silently dropped as no work", rows, err)
	}
}
