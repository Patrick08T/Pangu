//go:build integration

package scheduler

import (
	"context"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func maintenanceSibling(t *testing.T, original evolutionFixture) evolutionFixture {
	t.Helper()
	f := original
	f.scope.ProjectID = uuid.NewString()
	f.request.ProjectID = f.scope.ProjectID
	f.request.ID = uuid.NewString()
	maintenanceProject(t, f.scope.Namespace, f.scope.ProjectID)
	env, err := f.service.RegisterEnvironment(context.Background(), f.scope, evolution.Environment{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", ConnectionVersion: f.request.Payload.Agent.ConnectionVersion, Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.request.EnvironmentID = env.ID
	f.suite = f.createSuite(t)
	f.configurePolicy(t)
	return f
}

func maintenanceIntentState(t *testing.T, id string) string {
	t.Helper()
	var state string
	if err := entryQuery(t, `SELECT state FROM pangu.evolution_intents WHERE id=$1`, id).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestExperienceMaintenanceDriverBoundsRoundAndAdvancesAfterTimeout(t *testing.T) {
	b := maintenanceBackend(t)
	original := maintenanceEvolutionFixture(t, b)
	ctx := context.Background()
	ids := make([]string, 9)
	for i := range ids {
		f := original
		if i > 0 {
			f = maintenanceSibling(t, original)
		}
		f.request.ID = fmt.Sprintf("%08x-0000-0000-0000-%012x", i+1, uuid.New().ID())
		intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = intent.ID
		entryExec(t, `UPDATE pangu.evolution_intents SET state='reconciling' WHERE id=$1`, intent.ID)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = experience.Lock(ctx, tx, original.scope.Namespace, original.scope.ProjectID); err != nil {
		t.Fatal(err)
	}
	cursor := ""
	if err = b.driveExperienceMaintenance(ctx, b.Namespace(), &cursor); err == nil {
		t.Fatal("first timeout vanished")
	}
	if maintenanceIntentState(t, ids[0]) != "reconciling" {
		t.Fatal("blocked first intent changed")
	}
	if maintenanceIntentState(t, ids[7]) != "insufficient" {
		t.Fatal("first timeout starved same-page intent")
	}
	if maintenanceIntentState(t, ids[8]) != "reconciling" {
		t.Fatal("driver exceeded bounded round")
	}
	if cursor != ids[7] {
		t.Fatalf("cursor did not advance after each attempt: %s", cursor)
	}
	if err = b.driveExperienceMaintenance(ctx, b.Namespace(), &cursor); err != nil {
		t.Fatal(err)
	}
	if maintenanceIntentState(t, ids[8]) != "insufficient" || cursor != "" {
		t.Fatal("later round did not progress/wrap")
	}
}
