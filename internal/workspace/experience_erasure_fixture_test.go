//go:build integration

package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type erasureWorkspaceFixture struct {
	conn                                                     *pgx.Conn
	ns, project, request, run, node, report, root, directory string
}

func newErasureWorkspaceFixture(t *testing.T) erasureWorkspaceFixture {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("requires independent test database")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	f := erasureWorkspaceFixture{conn: conn, ns: "erasure-workspace-" + uuid.NewString(), project: uuid.NewString(), request: uuid.NewString(), run: uuid.NewString(), node: uuid.NewString(), report: uuid.NewString(), root: t.TempDir()}
	f.directory = filepath.Join(f.root, f.run)
	t.Setenv("PANGU_WORKSPACE_ROOT", f.root)
	t.Setenv("PANGU_WORKER_ID", "erasure-test")
	if err = os.Mkdir(f.directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(f.directory, "private.txt"), []byte("private-memory-derived-copy"), 0600); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'erasure','unused','main','erasure-test','git')`, f.project, f.ns)
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id) VALUES($1::uuid,$2,$3,'submit',$1::text,$1::text,'{"purpose":"business","workflow":{"input":"private-snapshot"}}',$4)`, f.request, f.ns, f.project, f.run)
	f.exec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','erasure-test',$3,true)`, f.run, f.project, f.directory)
	f.seedPayloads(t)
	f.receipt(t)
	return f
}

func (f erasureWorkspaceFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.conn.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f erasureWorkspaceFixture) seedPayloads(t *testing.T) {
	t.Helper()
	line := map[string]any{"version": 1, "artifact_id": f.node, "input_hash": strings.Repeat("a", 64), "output_hash": strings.Repeat("b", 64), "execution_input_hash": strings.Repeat("c", 64)}
	output, _ := json.Marshal(map[string]any{"lineage": line, "run_id": f.node, "report_ids": []string{f.report}, "report_sha256": strings.Repeat("d", 64), "text": "private-output", "iteration": map[string]any{"private": "iteration"}, "blockers": []string{"private-blocker"}})
	f.exec(t, `INSERT INTO pangu.workspace_checkpoints(node_run_id,run_id,commit_id,output,evidence) VALUES($1,$2,'commit',$3,'[{"private":"evidence"}]')`, f.node, f.run, output)
	f.exec(t, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence,metadata) VALUES($1,$2,$3,'private-title','private-body',$4,'[{"private":"evidence"}]','{"private":"metadata"}')`, f.report, f.run, f.node, output)
	f.exec(t, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt,status,finished_at,message,evidence) VALUES($1,$2,'answer',1,'succeeded',now(),'private-message','[{"private":"evidence"}]')`, f.run, f.node)
	f.exec(t, `INSERT INTO pangu.run_cancellations(run_id,namespace,reconciled_at) VALUES($1,$2,now())`, f.run, f.ns)
}

func (f erasureWorkspaceFixture) receipt(t *testing.T) {
	t.Helper()
	finished := time.Now().Add(-time.Minute).UTC()
	summary, _ := json.Marshal(map[string]any{"id": f.run, "project_id": f.project, "status": "succeeded", "stop_confirmed": true, "finished_at": finished, "nodes": []any{map[string]any{"id": f.node, "status": "succeeded"}}})
	f.exec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,$3,$4)`, f.run, f.ns, finished, summary)
}
