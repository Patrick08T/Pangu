//go:build integration

package scheduler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func maintenanceProject(t *testing.T, namespace, project string) {
	t.Helper()
	entryExec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'maintenance','unused','main','test','git')`, project, namespace)
}

func maintenanceMemory(t *testing.T, namespace, project string, expired, body, state bool) string {
	t.Helper()
	id, hours := uuid.NewString(), 24
	if expired {
		hours = -1
	}
	entryExec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by) VALUES($1,$2,$3,$1,1,$4,'[{}]',clock_timestamp()+$5::interval,'maintenance-test')`, id, namespace, project, strings.Repeat("a", 64), fmt.Sprintf("%d hours", hours))
	if state {
		entryExec(t, `INSERT INTO pangu.experience_states(version_id,validity) VALUES($1,'candidate')`, id)
	}
	if body {
		entryExec(t, `INSERT INTO pangu.experience_bodies(version_id,content) VALUES($1,'{}')`, id)
	}
	return id
}

func maintenanceMemoryState(t *testing.T, id string) (bool, bool) {
	t.Helper()
	var body, tombstone bool
	if err := entryQuery(t, `SELECT EXISTS(SELECT 1 FROM pangu.experience_bodies WHERE version_id=$1),coalesce((SELECT tombstoned FROM pangu.experience_states WHERE version_id=$1),false)`, id).Scan(&body, &tombstone); err != nil {
		t.Fatal(err)
	}
	return body, tombstone
}

func TestExperienceMaintenancePurgePreviewAndPolicyOff(t *testing.T) {
	entryDB(t)
	ns, project := "maintenance-"+uuid.NewString(), uuid.NewString()
	maintenanceProject(t, ns, project)
	expired := maintenanceMemory(t, ns, project, true, true, true)
	live := maintenanceMemory(t, ns, project, false, true, true)
	entryExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,1,'disabled-actor','{"enabled":false,"read_enabled":false}')`, ns, project)
	cursor := ""
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, true); err != nil {
		t.Fatal(err)
	}
	if body, tombstone := maintenanceMemoryState(t, expired); !body || tombstone || cursor != "" {
		t.Fatal("preview mutated memory or traversal")
	}
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, false); err != nil {
		t.Fatal(err)
	}
	if body, tombstone := maintenanceMemoryState(t, expired); body || !tombstone {
		t.Fatal("expired body not erased under disabled policy")
	}
	if body, tombstone := maintenanceMemoryState(t, live); !body || tombstone {
		t.Fatal("policy off erased a still-live body")
	}
}

func TestExperienceMaintenancePurgeRotatesPastProjectFailure(t *testing.T) {
	entryDB(t)
	ns := "maintenance-" + uuid.NewString()
	var last string
	for i := 1; i <= 9; i++ {
		project := fmt.Sprintf("%08x-0000-0000-0000-%012x", i, uuid.New().ID())
		maintenanceProject(t, ns, project)
		last = maintenanceMemory(t, ns, project, true, true, i != 1)
	}
	cursor := ""
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, false); err == nil {
		t.Fatal("corrupt first project was silently successful")
	}
	if body, _ := maintenanceMemoryState(t, last); !body {
		t.Fatal("unbounded first project page")
	}
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, false); err != nil {
		t.Fatal(err)
	}
	if body, tombstone := maintenanceMemoryState(t, last); body || !tombstone {
		t.Fatal("first project error starved later project")
	}
	if cursor != "" {
		t.Fatal("finished project traversal did not wrap")
	}
}

func TestExperienceMaintenancePurgeTombstonesWithoutBodiesAndHonorsNamespace(t *testing.T) {
	entryDB(t)
	ns, other := "maintenance-"+uuid.NewString(), "foreign-"+uuid.NewString()
	p, q := uuid.NewString(), uuid.NewString()
	maintenanceProject(t, ns, p)
	maintenanceProject(t, other, q)
	missing := maintenanceMemory(t, ns, p, true, false, true)
	foreign := maintenanceMemory(t, other, q, true, true, true)
	cursor := ""
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, false); err != nil {
		t.Fatal(err)
	}
	if _, tombstone := maintenanceMemoryState(t, missing); !tombstone {
		t.Fatal("zero body count was treated as drained")
	}
	if body, tombstone := maintenanceMemoryState(t, foreign); !body || tombstone {
		t.Fatal("cross-namespace erasure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := purgeExperienceMemory(ctx, ns, &cursor, false); err == nil {
		t.Fatal("cancelled maintenance treated as success")
	}
}

func maintenanceChecker(t *testing.T, namespace, project string) string {
	t.Helper()
	actor, id := uuid.NewString(), uuid.NewString()
	entryExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, actor, namespace)
	entryExec(t, `INSERT INTO pangu.learning_checkers(id,namespace,project_id,content_hash,config_hash,actor_id,expires_at) VALUES($1,$2,$3,$4,$4,$5,clock_timestamp()-interval '1 hour')`, id, namespace, project, strings.Repeat("a", 64), actor)
	entryExec(t, `INSERT INTO pangu.learning_checker_bodies(checker_id,body) VALUES($1,'{}')`, id)
	return id
}

func TestExperienceMaintenancePurgeCheckerOnlyProject(t *testing.T) {
	entryDB(t)
	ns, project := "maintenance-"+uuid.NewString(), uuid.NewString()
	maintenanceProject(t, ns, project)
	id := maintenanceChecker(t, ns, project)
	cursor := ""
	if err := purgeExperienceMemory(context.Background(), ns, &cursor, false); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var present bool
	if err = conn.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pangu.learning_checker_bodies WHERE checker_id=$1)`, id).Scan(&present); err != nil || present {
		t.Fatalf("checker-only project not purged: %v %v", present, err)
	}
}
