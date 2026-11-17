package schedule

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestRemoteEvolutionOwnedCurrentIntentIsExcluded(t *testing.T) {
	for _, status := range []string{"pending", "running", "unknown"} {
		t.Run(status, func(t *testing.T) {
			ctx, namespace, plan, conn := remoteEvolutionFixture(t)
			key := remoteEvolutionIntent(t, conn, namespace, plan, status)
			ids, err := RemotePending(ctx, namespace)
			if err != nil || slices.Contains(ids, plan.ID) {
				t.Errorf("evolution-owned plan listed: %v %v", ids, err)
			}
			result, err := ReconcileRemote(ctx, namespace, plan.ID, func(context.Context, *Plan) (string, error) {
				t.Error("ordinary reconciler sent evolution-owned intent")
				return uuid.NewString(), nil
			})
			if !errors.Is(err, ErrConflict) || result == nil || result.Ready {
				t.Errorf("evolution-owned intent confirmed: %+v %v", result, err)
			}
			var saved string
			if err := conn.QueryRow(ctx, `SELECT status FROM pangu.evolution_remote_intents WHERE effect_key=$1`, key).Scan(&saved); err != nil || saved != status {
				t.Fatalf("owner outbox changed: %s %v", saved, err)
			}
			if remoteEvolutionCurrent(t, ctx, namespace, plan.ID).Ready {
				t.Fatal("ordinary path stole owner receipt")
			}
		})
	}
}

func TestRemoteEvolutionObsoleteIntentsDoNotBlockCurrent(t *testing.T) {
	cases := []struct{ name, change string }{
		{"confirmed", `status='confirmed'`}, {"conflict", `status='conflict'`},
		{"old_revision", `revision=revision-1`}, {"other_namespace", `namespace=namespace||'-other'`},
		{"other_target", `target_id=gen_random_uuid()`},
		{"other_project", `project_id=(SELECT id FROM pangu.projects WHERE namespace=$2 AND id<>$3 LIMIT 1)`},
		{"snapshot_revision", `result=jsonb_set(result,'{revision}','1')`},
		{"snapshot_project", `result=jsonb_set(result,'{project_id}',to_jsonb(gen_random_uuid()::text))`},
		{"snapshot_target", `result=jsonb_set(result,'{target,id}',to_jsonb(gen_random_uuid()::text))`},
		{"malformed_target", `result=jsonb_set(result,'{target,id}','"not-a-uuid"')`},
		{"missing_target", `result=result-'target'`},
		{"snapshot_kind", `result=jsonb_set(result,'{target,kind}','"webhook"')`},
		{"snapshot_version", `result=jsonb_set(result,'{version_id}',to_jsonb(gen_random_uuid()::text))`},
		{"snapshot_owner", `result=jsonb_set(result,'{owner_id}',to_jsonb(gen_random_uuid()::text))`},
		{"snapshot_enabled", `result=jsonb_set(result,'{enabled}','false')`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, namespace, plan, conn := remoteEvolutionFixture(t)
			other := uuid.NewString()
			remoteEvolutionExec(t, conn, `INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,$2,'other','unused','main','test')`, other, namespace)
			key := remoteEvolutionIntent(t, conn, namespace, plan, "unknown")
			// Keep cleanup scoped even when testing a foreign namespace intent.
			defer remoteEvolutionExec(t, conn, `DELETE FROM pangu.evolution_remote_intents WHERE effect_key=$1`, key)
			remoteEvolutionExec(t, conn, `UPDATE pangu.evolution_remote_intents SET `+tc.change+` WHERE effect_key=$1 AND $2::text<>'' AND $3::uuid IS NOT NULL`, key, namespace, plan.Workflow.ProjectID)
			ids, err := RemotePending(ctx, namespace)
			if err != nil || !slices.Contains(ids, plan.ID) {
				t.Fatalf("unrelated intent blocked listing: %v %v", ids, err)
			}
			remote := uuid.NewString()
			result, err := ReconcileRemote(ctx, namespace, plan.ID, func(context.Context, *Plan) (string, error) { return remote, nil })
			if err != nil || result == nil || !result.Ready || result.HatchetID != remote {
				t.Fatalf("unrelated intent blocked current reconciliation: %+v %v", result, err)
			}
		})
	}
}

func TestRemoteEvolutionReceiptRechecksOwnership(t *testing.T) {
	ctx, namespace, plan, conn := remoteEvolutionFixture(t)
	result, err := ReconcileRemote(ctx, namespace, plan.ID, func(context.Context, *Plan) (string, error) {
		remoteEvolutionIntent(t, conn, namespace, plan, "pending")
		return uuid.NewString(), nil
	})
	if !errors.Is(err, ErrConflict) || result == nil || result.Ready {
		t.Fatalf("new owner receipt stolen: %+v %v", result, err)
	}
	if remoteEvolutionCurrent(t, ctx, namespace, plan.ID).Ready {
		t.Fatal("newly owned schedule marked synced")
	}
}
