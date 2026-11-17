package scheduler

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type entryFixture struct {
	ctx     context.Context
	actor   identity.Actor
	project string
	version *catalog.Version
	policy  experience.Policy
}

func entryDB(t *testing.T) {
	t.Helper()
	raw := os.Getenv("PANGU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable test database")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Port() != "55439" {
		t.Fatal("entry tests require disposable database 127.0.0.1:55439")
	}
	t.Setenv("PANGU_DATABASE_URL", raw)
	if err = database.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func entryExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func newEntryFixture(t *testing.T) entryFixture {
	t.Helper()
	entryDB(t)
	f := entryFixture{actor: identity.Actor{ID: uuid.NewString(), Namespace: "entry-" + uuid.NewString()}, project: uuid.NewString()}
	f.ctx = identity.WithActor(context.Background(), f.actor)
	entryExec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'entry','unused','main','test','git')`, f.project, f.actor.Namespace)
	entryExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, f.actor.ID, f.actor.Namespace)
	entryExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role,enabled) VALUES($1,$2,'manager',true)`, f.actor.ID, f.project)
	node := workflow.Node{ID: "answer", Executor: "codex", Instruction: "inspect only", Handoff: &workflow.HandoffContract{Objective: "inspect", Inputs: "task", Deliverables: "report", Acceptance: "evidence", FailureHandling: "report"}}
	object, err := catalog.Create(f.ctx, f.actor.Namespace, catalog.NewObject{Kind: "agent", Name: "entry agent", Spec: catalog.Spec{Agent: &node}})
	if err != nil {
		t.Fatal(err)
	}
	f.version, err = catalog.Publish(f.ctx, f.actor.Namespace, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	f.policy = experience.Policy{ProjectID: f.project, ActorID: f.actor.ID, Revision: 1, Enabled: true, AutoPublish: true, Entrypoints: []string{"workbench:main"}}
	f.savePolicy(t)
	entryExec(t, `INSERT INTO pangu.evolution_bindings(namespace,project_id,entry_key,adopted_version,stable_version) VALUES($1,$2,'workbench:main',$3,$3)`, f.actor.Namespace, f.project, f.version.ID)
	return f
}

func (f entryFixture) savePolicy(t *testing.T) {
	t.Helper()
	body, err := json.Marshal(f.policy)
	if err != nil {
		t.Fatal(err)
	}
	entryExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,$3,$4,$5)`, f.actor.Namespace, f.project, f.policy.Revision, f.policy.ActorID, body)
}

func entryQuery(t *testing.T, sql string, args ...any) pgx.Row {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn.QueryRow(context.Background(), sql, args...)
}
