package schedule

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Use the provisioned disposable database; these tests never apply migrations.
func remoteEvolutionFixture(t *testing.T) (context.Context, string, *Plan, *pgx.Conn) {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires an independent PostgreSQL test database")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	namespace, project, id := "remote-evolution-"+uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		_, err := conn.Exec(ctx, `DELETE FROM pangu.evolution_remote_intents WHERE namespace=$1`, namespace)
		if err != nil {
			t.Error(err)
		}
		_, err = conn.Exec(ctx, `DELETE FROM pangu.schedules WHERE namespace=$1`, namespace)
		if err != nil {
			t.Error(err)
		}
		_, err = conn.Exec(ctx, `DELETE FROM pangu.projects WHERE namespace=$1`, namespace)
		if err != nil {
			t.Error(err)
		}
		_ = conn.Close(ctx)
	})
	remoteEvolutionExec(t, conn, `INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,$2,'remote test','unused','main','test')`, project, namespace)
	definition := workflow.Definition{ProjectID: project, VersionID: uuid.NewString(), Nodes: []workflow.Node{{ID: "test", Executor: "eino", Model: "test", Instruction: "test"}}}
	remoteEvolutionExec(t, conn, `INSERT INTO pangu.schedules(id,namespace,project_id,name,expression,workflow,revision) VALUES($1::uuid,$2,$3,$1::text,'* * * * *',$4,2)`, id, namespace, project, remoteEvolutionJSON(t, definition))
	plan, err := Get(ctx, namespace, id)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, namespace, plan, conn
}

func remoteEvolutionJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func remoteEvolutionExec(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := conn.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

func remoteEvolutionIntent(t *testing.T, conn *pgx.Conn, namespace string, plan *Plan, status string) string {
	t.Helper()
	result := map[string]any{"target": map[string]any{"kind": "cron", "id": plan.ID}, "project_id": plan.Workflow.ProjectID,
		"version_id": plan.Workflow.VersionID, "revision": plan.Revision, "owner_id": plan.CreatedBy, "enabled": !plan.Paused && !plan.Deleted, "remote_state": "pending"}
	after := remoteEvolutionJSON(t, result)
	result["revision"], result["version_id"], result["remote_state"] = plan.Revision-1, uuid.NewString(), "confirmed"
	key := uuid.NewString() + ":1:cron:" + plan.ID + ":adopt"
	remoteEvolutionExec(t, conn, `INSERT INTO pangu.evolution_remote_intents(effect_key,namespace,project_id,target_id,revision,expected,result,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, key, namespace, plan.Workflow.ProjectID, plan.ID, plan.Revision, remoteEvolutionJSON(t, result), after, status)
	return key
}

func remoteEvolutionCurrent(t *testing.T, ctx context.Context, namespace, id string) *Plan {
	t.Helper()
	plan, err := Get(ctx, namespace, id)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
