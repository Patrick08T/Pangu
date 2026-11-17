//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
)

func maintenanceEvolutionFixture(t *testing.T, b Backend) evolutionFixture {
	t.Helper()
	entryDB(t)
	t.Setenv("PANGU_SECRET_EVOLUTION_DRIVER_TEST", "test-not-a-secret")
	f := evolutionFixture{scope: evolution.Scope{Namespace: b.Namespace(), ProjectID: uuid.NewString(), ActorID: uuid.NewString()}, service: b.EvolutionService()}
	maintenanceProject(t, f.scope.Namespace, f.scope.ProjectID)
	entryExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, f.scope.ActorID, f.scope.Namespace)
	f.configure(t, "http://127.0.0.1:1")
	f.suite = f.createSuite(t)
	f.configurePolicy(t)
	return f
}

func TestExperienceMaintenanceRetiresHistoricalExpiredIntent(t *testing.T) {
	b := maintenanceBackend(t)
	f := maintenanceEvolutionFixture(t, b)
	intent, err := f.service.ReserveCandidate(context.Background(), f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	entryExec(t, `UPDATE pangu.evolution_intents SET state='invalidated',created_at=clock_timestamp()-interval '2 hours' WHERE id=$1`, intent.ID)
	cursor := ""
	if err = b.retireExperienceMaintenance(context.Background(), &cursor); err != nil {
		t.Fatal(err)
	}
	var bodies int
	if err = entryQuery(t, `SELECT count(*) FROM pangu.catalog_experiment_candidates WHERE experiment_id=$1 AND payload IS NOT NULL`, intent.ExperimentID).Scan(&bodies); err != nil || bodies != 0 {
		t.Fatalf("historical candidate body not retired: %d %v", bodies, err)
	}
	var payloadCleared bool
	if err = entryQuery(t, `SELECT request#>'{request,payload,agent}'='{}'::jsonb FROM pangu.evolution_intents WHERE id=$1`, intent.ID).Scan(&payloadCleared); err != nil || !payloadCleared {
		t.Fatalf("historical private request payload remained: %v %v", payloadCleared, err)
	}
}

func TestExperienceMaintenanceDrivesRecoveryWithAcceptedActor(t *testing.T) {
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	intent, err := f.service.ReserveCandidate(context.Background(), f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	entryExec(t, `UPDATE pangu.evolution_intents SET state='reconciling' WHERE id=$1`, intent.ID)
	// A later policy editor must never replace the actor accepted by the intent.
	entryExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT namespace,project_id,2,$2,jsonb_set(jsonb_set(policy,'{actor_id}',to_jsonb($2::text)),'{revision}','2') FROM pangu.experience_policies WHERE namespace=$1 AND revision=1`, f.scope.Namespace, uuid.NewString())
	cursor := ""
	if err = (Backend{}).driveExperienceMaintenance(context.Background(), f.scope.Namespace, &cursor); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = entryQuery(t, `SELECT state FROM pangu.evolution_intents WHERE id=$1`, intent.ID).Scan(&state); err != nil || state != "insufficient" {
		t.Fatalf("recovery was not driven with accepted identity: %s %v", state, err)
	}
	var claims int
	if err = entryQuery(t, `SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1`, intent.ID).Scan(&claims); err != nil || claims != 0 {
		t.Fatalf("real recovery did not release stopped empty experiment: %d %v", claims, err)
	}
}

func TestExperienceMaintenanceDriverDoesNotCrossNamespace(t *testing.T) {
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	intent, err := f.service.ReserveCandidate(context.Background(), f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	entryExec(t, `UPDATE pangu.evolution_intents SET state='reconciling' WHERE id=$1`, intent.ID)
	cursor := ""
	if err = (Backend{}).driveExperienceMaintenance(context.Background(), "foreign-"+uuid.NewString(), &cursor); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = entryQuery(t, `SELECT state FROM pangu.evolution_intents WHERE id=$1`, intent.ID).Scan(&state); err != nil || state != "reconciling" {
		t.Fatalf("foreign intent changed: %s %v", state, err)
	}
}
