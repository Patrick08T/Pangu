//go:build integration

package scheduler

import (
	"context"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func TestExperienceMaintenancePurgeSlowProjectCannotConsumeLaterBudget(t *testing.T) {
	entryDB(t)
	ctx := context.Background()
	ns := "maintenance-" + uuid.NewString()
	first := fmt.Sprintf("00000001-0000-0000-0000-%012x", uuid.New().ID())
	last := fmt.Sprintf("00000002-0000-0000-0000-%012x", uuid.New().ID())
	maintenanceProject(t, ns, first)
	maintenanceProject(t, ns, last)
	blocked := maintenanceMemory(t, ns, first, true, true, true)
	later := maintenanceMemory(t, ns, last, true, true, true)
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
	if err = experience.Lock(ctx, tx, ns, first); err != nil {
		t.Fatal(err)
	}
	cursor := ""
	if err = purgeExperienceMemory(ctx, ns, &cursor, false); err == nil {
		t.Fatal("blocked project timeout disappeared")
	}
	if body, tombstone := maintenanceMemoryState(t, blocked); !body || tombstone {
		t.Fatal("locked project partially erased")
	}
	if body, tombstone := maintenanceMemoryState(t, later); body || !tombstone {
		t.Fatal("slow first project starved independent later budget")
	}
}

func TestExperienceMaintenancePurgeLimitsBodiesPerProjectRound(t *testing.T) {
	entryDB(t)
	ns, project := "maintenance-"+uuid.NewString(), uuid.NewString()
	maintenanceProject(t, ns, project)
	for i := 0; i < 33; i++ {
		maintenanceMemory(t, ns, project, true, true, true)
	}
	cursor := ""
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, false); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := entryQuery(t, `SELECT count(*) FROM pangu.experience_bodies b JOIN pangu.experience_versions v ON v.id=b.version_id WHERE v.namespace=$1 AND v.project_id=$2`, ns, project).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("unbounded project round: %d %v", remaining, err)
	}
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, false); err != nil {
		t.Fatal(err)
	}
	if err := entryQuery(t, `SELECT count(*) FROM pangu.experience_bodies b JOIN pangu.experience_versions v ON v.id=b.version_id WHERE v.namespace=$1 AND v.project_id=$2`, ns, project).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("later round did not finish pending body: %d %v", remaining, err)
	}
}
