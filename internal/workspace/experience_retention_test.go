//go:build integration

package workspace

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// This fixture deliberately never migrates: the coordinator owns the disposable schema.
func experienceRetentionTx(t *testing.T) (pgx.Tx, string, string) {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires independent test database")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	project, actor, request, run := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	ns := "retention-purge-" + uuid.NewString()
	experienceRetentionExec(t, tx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'retention','unused','main','test','git')`, project, ns)
	experienceRetentionExec(t, tx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, actor, ns)
	snapshot := `{"purpose":"evaluation","entry_key":"workbench:main","entry_revision":7,"created_by":"` + actor + `","workflow":{"input":"private-input","nodes":[{"instruction":"private-instruction"}],"project_id":"` + project + `","version_id":"audit-version","budget":{"max_calls":2}},"fork":{"source_run_id":"audit-source","reused":{"node":"private-copy"}}}`
	experienceRetentionExec(t, tx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,task_id,input_hash) VALUES($1::uuid,$2,$3,'evolution',$1::text,$1::text,$4,$5,$1::uuid,$6)`, request, ns, project, snapshot, run, strings.Repeat("a", 64))
	experienceRetentionSeedPlan(t, tx, request, run, ns, project, actor)
	return tx, request, run
}

func experienceRetentionExec(t *testing.T, tx pgx.Tx, query string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func experienceRetentionSeedPlan(t *testing.T, tx pgx.Tx, request, run, ns, project, actor string) {
	t.Helper()
	intent, caseID := uuid.NewString(), uuid.NewString()
	hash := strings.Repeat("a", 64)
	experienceRetentionExec(t, tx, `INSERT INTO pangu.evolution_intents(id,namespace,project_id,policy_revision,state,request_hash,request,targets) VALUES($1,$2,$3,1,'rejected',$4,'{}','[]')`, intent, ns, project, hash)
	experienceRetentionExec(t, tx, `INSERT INTO pangu.evolution_executions(intent_id,generation,arm,case_id,repeat,input_hash,request_id,run_id,status,max_calls,max_tokens) VALUES($1,1,'candidate',$2,1,$3,$4,$5,'completed',1,1)`, intent, caseID, hash, request, run)
	experienceRetentionExec(t, tx, `INSERT INTO pangu.evolution_case_plans(request_id,namespace,project_id,actor_id,plan,plan_hash) VALUES($1,$2,$3,$4,'{"private":"plan-body"}',$5)`, request, ns, project, actor, hash)
	experienceRetentionExec(t, tx, `INSERT INTO pangu.evolution_case_runtime(request_id,run_id,status,outputs,finished_at,stop_confirmed) VALUES($1,$2,'completed','{"private":"runtime-output"}',now(),true)`, request, run)
	experienceRetentionExec(t, tx, `INSERT INTO pangu.evolution_node_runs(request_id,node_key,input_hash,node_run_id,status,output,finished_at) VALUES($1,'answer',$2,$3,'completed','{"private":"node-output"}',now())`, request, hash, uuid.NewString())
}

func TestExperienceRetentionPurgePreservesMetadataAndErasesBodies(t *testing.T) {
	tx, request, run := experienceRetentionTx(t)
	experienceRetentionExec(t, tx, `SELECT pangu.purge_run_payloads($1)`, run)
	var purpose, entry string
	var revision int
	var snapshot, plan, runtime, node, digest, tombstone bool
	err := tx.QueryRow(context.Background(), `SELECT coalesce(s.snapshot->>'purpose',''),coalesce(s.snapshot->>'entry_key',''),
	 coalesce((s.snapshot->>'entry_revision')::int,0),NOT (s.snapshot->'workflow' ? 'input'),p.plan IS NULL,
	 r.outputs IS NULL,n.output IS NULL,p.plan_hash=$2,p.erased_at IS NOT NULL
	 FROM pangu.submissions s JOIN pangu.evolution_case_plans p ON p.request_id=s.id
	 JOIN pangu.evolution_case_runtime r ON r.request_id=p.request_id
	 JOIN pangu.evolution_node_runs n ON n.request_id=p.request_id WHERE s.id=$1`, request, strings.Repeat("a", 64)).Scan(&purpose, &entry, &revision, &snapshot, &plan, &runtime, &node, &digest, &tombstone)
	if err != nil {
		t.Fatalf("purge must preserve purpose and trusted entry identity: %v", err)
	}
	if purpose != "evaluation" || entry != "workbench:main" || revision != 7 {
		t.Errorf("metadata lost: purpose=%q entry=%q revision=%d", purpose, entry, revision)
	}
	if !snapshot || !plan || !runtime || !node || !digest || !tombstone {
		t.Fatalf("private bodies/tombstone: snapshot=%v plan=%v runtime=%v node=%v hash=%v erased=%v", snapshot, plan, runtime, node, digest, tombstone)
	}
	experienceRetentionExec(t, tx, `SELECT pangu.purge_run_payloads($1)`, run)
}
