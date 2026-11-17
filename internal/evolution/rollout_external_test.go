//go:build integration

package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type rolloutEntries struct {
	id, actor string
	fail      bool
}

func (a rolloutEntries) ReadTx(ctx context.Context, tx pgx.Tx, scope Scope, target EntryTarget) (EntrySnapshot, error) {
	var data []byte
	var entry EntrySnapshot
	if scope.ActorID != a.actor {
		return entry, ErrForbidden
	}
	if err := tx.QueryRow(ctx, `SELECT request->'_testEntry' FROM pangu.evolution_intents WHERE id=$1 AND namespace=$2 AND project_id=$3`, a.id, scope.Namespace, scope.ProjectID).Scan(&data); err != nil {
		return entry, err
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return entry, err
	}
	if entry.Target != target {
		return entry, ErrForbidden
	}
	return entry, nil
}

func (a rolloutEntries) CompareAndSwapTx(ctx context.Context, tx pgx.Tx, scope Scope, key string, before EntrySnapshot, next string) (EntrySnapshot, error) {
	current, err := a.ReadTx(ctx, tx, scope, before.Target)
	if err != nil {
		return current, err
	}
	if !strings.Contains(key, ":1:"+before.Target.Key()+":") {
		return current, errors.New("幂等键缺少generation")
	}
	if !rolloutSameEntry(current, before) {
		return current, ErrConflict
	}
	current.VersionID = next
	current.Revision++
	current.RemoteState = "pending"
	_, err = tx.Exec(ctx, `UPDATE pangu.evolution_intents SET request=jsonb_set(jsonb_set(request,'{_testEntry}',$2),'{_testCalls}',to_jsonb(coalesce((request->>'_testCalls')::int,0)+1)) WHERE id=$1`, a.id, jsonBytes(current))
	if err != nil {
		return current, err
	}
	if a.fail {
		return current, errors.New("事务内入口适配器故障")
	}
	return current, nil
}

func rolloutWithExternal(t *testing.T) (rolloutFixture, rolloutEntries) {
	t.Helper()
	f := newRolloutFixture(t)
	adapter := rolloutEntries{id: f.id, actor: f.scope.ActorID}
	target := EntryTarget{Kind: "cron", ID: "daily"}
	f.policy.Revision++
	f.policy.Entrypoints = append(f.policy.Entrypoints, target.Key())
	f.savePolicy(t)
	err := transaction(context.Background(), func(tx pgx.Tx) error {
		intent, err := readIntentTx(context.Background(), tx, f.scope, f.id)
		if err != nil {
			return err
		}
		intent.Config.Request.PolicyRevision = f.policy.Revision
		intent.Config.Request.Entries = append(intent.Config.Request.Entries, target)
		entry := EntrySnapshot{Target: target, ProjectID: f.scope.ProjectID, VersionID: f.request.BaselineVersion, Revision: 7, OwnerID: "different-owner", Enabled: true, RemoteState: "confirmed"}
		intent.Config.Entries = append(intent.Config.Entries, entry)
		targets := targetKeys(intent.Config.Request)
		if _, err = tx.Exec(context.Background(), `UPDATE pangu.evolution_intents SET policy_revision=$2,request=$3::jsonb||jsonb_build_object('_testEntry',$4::jsonb),targets=$5 WHERE id=$1`, f.id, f.policy.Revision, jsonBytes(intent.Config), jsonBytes(entry), jsonBytes(targets)); err != nil {
			return err
		}
		_, err = tx.Exec(context.Background(), `INSERT INTO pangu.evolution_target_claims(namespace,project_id,target,intent_id,generation) VALUES($1,$2,$3,$4,1)`, f.scope.Namespace, f.scope.ProjectID, "entry:"+target.Key(), f.id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f.service.Entries = adapter
	return f, adapter
}

func rolloutRemote(t *testing.T, f rolloutFixture, adapter rolloutEntries, state string) (EntrySnapshot, int) {
	t.Helper()
	var entry EntrySnapshot
	var calls int
	err := transaction(context.Background(), func(tx pgx.Tx) error {
		var err error
		entry, err = adapter.ReadTx(context.Background(), tx, f.scope, EntryTarget{Kind: "cron", ID: "daily"})
		if err != nil {
			return err
		}
		if state != "" {
			entry.RemoteState = state
			if _, err = tx.Exec(context.Background(), `UPDATE pangu.evolution_intents SET request=jsonb_set(request,'{_testEntry}',$2) WHERE id=$1`, f.id, jsonBytes(entry)); err != nil {
				return err
			}
		}
		return tx.QueryRow(context.Background(), `SELECT coalesce((request->>'_testCalls')::int,0) FROM pangu.evolution_intents WHERE id=$1`, f.id).Scan(&calls)
	})
	if err != nil {
		t.Fatal(err)
	}
	return entry, calls
}

func TestRolloutExternalPendingRetainsClaimsUntilRestoredConfirmation(t *testing.T) {
	f, adapter := rolloutWithExternal(t)
	ctx := context.Background()
	intent, err := f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "switching" {
		t.Fatal("pending adapter", intent, err)
	}
	remote, calls := rolloutRemote(t, f, adapter, "unknown")
	if calls != 1 || remote.VersionID != f.candidate {
		t.Fatal(remote, calls)
	}
	if _, err = f.service.RequestRollback(ctx, f.scope, f.id, 1, "stop trial"); err != nil {
		t.Fatal(err)
	}
	intent, err = f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "reconciling" {
		t.Fatal(intent, err)
	}
	_, claims := f.binding(t)
	_, calls = rolloutRemote(t, f, adapter, "")
	if claims != 3 || calls != 1 {
		t.Fatal("uncertain state released or blindly retried", claims, calls)
	}
	rolloutRemote(t, f, adapter, "confirmed")
	// 用新service/adapter模拟进程重启；恢复只依赖持久数据。
	service := Service{Entries: rolloutEntries{id: f.id, actor: f.scope.ActorID}}
	intent, err = service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "reconciling" {
		t.Fatal(intent, err)
	}
	remote, calls = rolloutRemote(t, f, adapter, "confirmed")
	if calls != 2 || remote.VersionID != f.request.BaselineVersion {
		t.Fatal("no bounded compensation", remote, calls)
	}
	intent, err = service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "rolled_back" {
		t.Fatal(intent, err)
	}
	_, claims = f.binding(t)
	if claims != 0 {
		t.Fatal("confirmed compensation retained claims", claims)
	}
}

func TestRolloutAdapterFailureRollsBackAllLocalWrites(t *testing.T) {
	f, adapter := rolloutWithExternal(t)
	adapter.fail = true
	f.service.Entries = adapter
	if _, err := f.service.Reconcile(context.Background(), f.scope, f.id, 1); err == nil {
		t.Fatal("adapter failure accepted")
	}
	b, claims := f.binding(t)
	remote, calls := rolloutRemote(t, f, adapter, "")
	if b.Revision != 1 || b.AdoptedVersion != f.request.BaselineVersion || claims != 3 || calls != 0 || remote.VersionID != f.request.BaselineVersion {
		t.Fatal("partial transaction persisted", b, claims, remote, calls)
	}
}
