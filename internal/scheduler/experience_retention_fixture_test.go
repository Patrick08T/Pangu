//go:build integration

package scheduler

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type retentionFixture struct {
	conn                                                            *pgx.Conn
	ns, project, actor, intent, request, run, caseID, source, suite string
}

func newRetentionFixture(t *testing.T, seconds ...int) retentionFixture {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires independent test database")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	f := retentionFixture{conn: conn, ns: "retention-" + uuid.NewString(), project: uuid.NewString(), actor: uuid.NewString(), intent: uuid.NewString(), request: uuid.NewString(), run: uuid.NewString(), caseID: uuid.NewString(), source: uuid.NewString(), suite: uuid.NewString()}
	f.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'retention','unused','main','test','git')`, f.project, f.ns)
	f.exec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, f.actor, f.ns)
	ttl := 3600
	if len(seconds) > 0 {
		ttl = seconds[0]
	}
	f.policy(t, 1, ttl, true)
	f.seedSuite(t)
	f.seedRun(t)
	return f
}

func (f retentionFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.conn.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f retentionFixture) policy(t *testing.T, revision, seconds int, enabled bool) {
	t.Helper()
	f.exec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,$3,$4,jsonb_build_object('enabled',$5::boolean,'observation',jsonb_build_object('retention_seconds',$6::bigint,'source_retention_authorized',true)))`, f.ns, f.project, revision, f.actor, enabled, seconds)
}

func (f retentionFixture) seedSuite(t *testing.T) {
	t.Helper()
	hash, pool, experiment := strings.Repeat("a", 64), uuid.NewString(), uuid.NewString()
	f.exec(t, `INSERT INTO pangu.evaluation_cases(id,namespace,project_id,actor_id,name,input,input_hash,expected,criteria) VALUES($1,$2,$3,$4,'private','private-input',$5,'private-answer','[]')`, f.caseID, f.ns, f.project, f.actor, hash)
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,task_id,input_hash) VALUES($1::uuid,$2,$3,'source',$1::text,$1::text,'{"workflow":{"input":"private-input"}}',$1::uuid,$1::uuid,$4)`, f.source, f.ns, f.project, hash)
	f.exec(t, `INSERT INTO pangu.evaluation_holdout_pools(id,namespace,project_id,holdout_hash,rules_hash,max_candidates) VALUES($1,$2,$3,$4,$4,1)`, pool, f.ns, f.project, hash)
	f.exec(t, `INSERT INTO pangu.evaluation_suites(id,namespace,project_id,pool_id,spec,spec_hash) VALUES($1,$2,$3,$4,'{}',$5)`, f.suite, f.ns, f.project, pool, hash)
	f.exec(t, `INSERT INTO pangu.evaluation_suite_cases(suite_id,case_id,partition,input_hash,family_id,task_id,source_run_id) VALUES($1,$2,'holdout',$3,'family',$4,$4)`, f.suite, f.caseID, hash, f.source)
	f.exec(t, `INSERT INTO pangu.evaluation_experiments(id,namespace,project_id,suite_id,generation,spec,spec_hash) VALUES($1,$2,$3,$4,1,'{}',$5)`, experiment, f.ns, f.project, f.suite, hash)
	f.exec(t, `INSERT INTO pangu.evolution_intents(id,namespace,project_id,policy_revision,state,request_hash,request,targets,experiment_id) VALUES($1,$2,$3,1,'rejected',$4,'{}','[]',$5)`, f.intent, f.ns, f.project, hash, experiment)
}

func (f retentionFixture) seedRun(t *testing.T) {
	t.Helper()
	hash := strings.Repeat("b", 64)
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,task_id,input_hash) VALUES($1::uuid,$2,$3,'evolution',$1::text,$1::text,'{"purpose":"evaluation","workflow":{"input":"private-case"}}',$4,$1::uuid,$5)`, f.request, f.ns, f.project, f.run, hash)
	f.exec(t, `INSERT INTO pangu.evolution_executions(intent_id,generation,arm,case_id,repeat,input_hash,request_id,run_id,status,max_calls,max_tokens) VALUES($1,1,'candidate',$2,(SELECT coalesce(max(repeat),0)+1 FROM pangu.evolution_executions WHERE intent_id=$1),$3,$4,$5,'completed',1,1)`, f.intent, f.caseID, hash, f.request, f.run)
	f.exec(t, `INSERT INTO pangu.evolution_case_plans(request_id,namespace,project_id,actor_id,plan,plan_hash,created_at) VALUES($1,$2,$3,$4,'{"private":"case-plan"}',$5,now()-interval '2 hours')`, f.request, f.ns, f.project, f.actor, hash)
	f.exec(t, `INSERT INTO pangu.evolution_case_runtime(request_id,run_id,status,outputs,finished_at,stop_confirmed) VALUES($1,$2,'completed','{"private":"runtime-output"}',now(),true)`, f.request, f.run)
	f.exec(t, `INSERT INTO pangu.evolution_node_runs(request_id,node_key,input_hash,node_run_id,status,output,finished_at) VALUES($1,'answer',$2,$3,'completed','{"private":"node-output"}',now())`, f.request, hash, uuid.NewString())
}

func (f retentionFixture) erased(t *testing.T) bool {
	t.Helper()
	var erased bool
	err := f.conn.QueryRow(context.Background(), `SELECT p.plan IS NULL AND p.erased_at IS NOT NULL AND r.outputs IS NULL AND n.output IS NULL AND s.purged_at IS NOT NULL AND s.snapshot->>'purpose'='evaluation' FROM pangu.evolution_case_plans p JOIN pangu.evolution_case_runtime r USING(request_id) JOIN pangu.evolution_node_runs n USING(request_id) JOIN pangu.submissions s ON s.id=p.request_id WHERE p.request_id=$1`, f.request).Scan(&erased)
	if err != nil {
		t.Fatal(err)
	}
	return erased
}
