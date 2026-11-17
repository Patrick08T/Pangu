//go:build integration

package scheduler

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
)

type evolutionStartCaller interface {
	StartEvolution(context.Context, evolution.Scope, evolution.CandidateRequest) (*evolution.Intent, error)
}

func evolutionStartInterface(t *testing.T, b Backend) evolutionStartCaller {
	t.Helper()
	starter, ok := any(b).(evolutionStartCaller)
	if !ok {
		t.Fatal("production Backend cannot durably enqueue evolution through the API contract")
	}
	return starter
}

func evolutionStartContext(f evolutionFixture) context.Context {
	return identity.WithActor(context.Background(), identity.Actor{ID: f.scope.ActorID, Namespace: f.scope.Namespace})
}

func evolutionStartEffects(t *testing.T, f evolutionFixture) [4]int {
	t.Helper()
	var out [4]int
	err := entryQuery(t, `SELECT
 (SELECT count(*) FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2),
 (SELECT count(*) FROM pangu.evolution_budget_charges WHERE namespace=$1 AND project_id=$2),
 (SELECT count(*) FROM pangu.evolution_target_claims WHERE namespace=$1 AND project_id=$2),
 (SELECT count(*) FROM pangu.evolution_executions e JOIN pangu.evolution_intents i ON i.id=e.intent_id WHERE i.namespace=$1 AND i.project_id=$2)`, f.scope.Namespace, f.scope.ProjectID).Scan(&out[0], &out[1], &out[2], &out[3])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEvolutionStartDurablyQueuesOnceWithoutDriving(t *testing.T) {
	var calls atomic.Int32
	b := maintenanceBackendHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	f := maintenanceEvolutionFixture(t, b)
	starter, ctx := evolutionStartInterface(t, b), evolutionStartContext(f)
	intent, err := starter.StartEvolution(ctx, f.scope, f.request)
	if err != nil || intent == nil || intent.ID != f.request.ID || intent.State != "evaluating" || intent.Generation != 1 || intent.ExperimentID == "" {
		t.Fatalf("durable acceptance: %+v %v", intent, err)
	}
	prior, err := starter.StartEvolution(ctx, f.scope, f.request)
	if err != nil || prior == nil || prior.ExperimentID != intent.ExperimentID || prior.RequestHash != intent.RequestHash {
		t.Fatalf("idempotent retry: %+v %v", prior, err)
	}
	if effects := evolutionStartEffects(t, f); effects != [4]int{1, 1, 3, 0} {
		t.Fatalf("reservation charged twice or drove execution: %v", effects)
	}
	f.request.Payload.Agent.Instruction = "changed after acceptance"
	if _, err = starter.StartEvolution(ctx, f.scope, f.request); !errors.Is(err, evolution.ErrConflict) {
		t.Fatalf("changed same-id request: %v", err)
	}
	items, err := evolutionPendingPage(ctx, b.Namespace(), "")
	if err != nil || len(items) != 1 || items[0].ID != intent.ID || items[0].Generation != 1 || items[0].Scope != f.scope {
		t.Fatalf("maintenance cannot discover original accepted identity: %+v %v", items, err)
	}
	if effects := evolutionStartEffects(t, f); effects != [4]int{1, 1, 3, 0} || calls.Load() != 0 {
		t.Fatalf("conflict or synchronous remote call changed effects: %v HTTP=%d", effects, calls.Load())
	}
}

func TestEvolutionStartRejectsUnreadyBackend(t *testing.T) {
	starter := evolutionStartInterface(t, Backend{})
	if intent, err := starter.StartEvolution(context.Background(), evolution.Scope{}, evolution.CandidateRequest{}); intent != nil || !errors.Is(err, ErrNotReady) {
		t.Fatalf("unready start must fail safely: %+v %v", intent, err)
	}
}

func TestEvolutionStartBindsHumanIdentityAndNamespace(t *testing.T) {
	for _, name := range []string{"anonymous", "token", "wrong_actor", "wrong_actor_namespace", "foreign_backend_namespace", "empty_actor"} {
		t.Run(name, func(t *testing.T) {
			b := maintenanceBackend(t)
			f := maintenanceEvolutionFixture(t, b)
			starter := evolutionStartInterface(t, b)
			scope := f.scope
			actor := identity.Actor{ID: f.scope.ActorID, Namespace: f.scope.Namespace}
			switch name {
			case "anonymous":
				actor = identity.Actor{}
			case "token":
				actor.TokenID = uuid.NewString()
			case "wrong_actor":
				actor.ID = uuid.NewString()
			case "wrong_actor_namespace":
				actor.Namespace = "foreign-" + uuid.NewString()
			case "foreign_backend_namespace":
				scope.Namespace = "foreign-" + uuid.NewString()
				actor.Namespace = scope.Namespace
			case "empty_actor":
				scope.ActorID = ""
			}
			ctx := identity.WithActor(context.Background(), actor)
			intent, err := starter.StartEvolution(ctx, scope, f.request)
			if intent != nil || !errors.Is(err, evolution.ErrForbidden) || evolutionStartEffects(t, f) != [4]int{} {
				t.Fatalf("unbound human reserved work: %+v %v", intent, err)
			}
		})
	}
}

func TestEvolutionStartInheritsCurrentPolicyAndManagerAuthority(t *testing.T) {
	for _, name := range []string{"disabled_policy", "revoked_policy", "disabled_user", "operator", "removed_member"} {
		t.Run(name, func(t *testing.T) {
			b := maintenanceBackend(t)
			f := maintenanceEvolutionFixture(t, b)
			starter := evolutionStartInterface(t, b)
			switch name {
			case "disabled_policy":
				entryExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT namespace,project_id,2,actor_id,jsonb_set(jsonb_set(policy,'{revision}','2'),'{enabled}','false') FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=1`, f.scope.Namespace, f.scope.ProjectID)
				f.request.PolicyRevision = 2
			case "revoked_policy":
				entryExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT namespace,project_id,2,actor_id,jsonb_set(jsonb_set(policy,'{revision}','2'),'{revoked}','true') FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=1`, f.scope.Namespace, f.scope.ProjectID)
				f.request.PolicyRevision = 2
			case "disabled_user":
				entryExec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.scope.ActorID)
			case "operator":
				entryExec(t, `UPDATE pangu.users SET is_admin=false WHERE id=$1`, f.scope.ActorID)
				entryExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role) VALUES($1,$2,'operator')`, f.scope.ActorID, f.scope.ProjectID)
			case "removed_member":
				entryExec(t, `UPDATE pangu.users SET is_admin=false WHERE id=$1`, f.scope.ActorID)
				entryExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role,enabled) VALUES($1,$2,'manager',false)`, f.scope.ActorID, f.scope.ProjectID)
			}
			intent, err := starter.StartEvolution(evolutionStartContext(f), f.scope, f.request)
			if intent != nil || !errors.Is(err, evolution.ErrForbidden) || evolutionStartEffects(t, f) != [4]int{} {
				t.Fatalf("current authority bypassed: %+v %v", intent, err)
			}
		})
	}
}
