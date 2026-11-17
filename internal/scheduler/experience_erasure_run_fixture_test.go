//go:build integration

package scheduler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (f erasureFixture) runCopies(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, f.run)
	t.Setenv("PANGU_WORKSPACE_ROOT", root)
	t.Setenv("PANGU_WORKER_ID", "test")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "derived.txt"), []byte("private-derived-file"), 0600); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','test',$3,true)`, f.run, f.project, directory)
	node, report := uuid.NewString(), uuid.NewString()
	output, _ := json.Marshal(map[string]any{"text": "private-derived-output", "run_id": node, "report_ids": []string{report}, "report_sha256": strings.Repeat("f", 64)})
	f.exec(t, `INSERT INTO pangu.workspace_checkpoints(node_run_id,run_id,commit_id,output,evidence) VALUES($1,$2,'commit',$3,'[{"private":"evidence"}]')`, node, f.run, output)
	f.exec(t, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence,metadata) VALUES($1,$2,$3,'private-title','private-body',$4,'[]','{"private":"metadata"}')`, report, f.run, node, output)
	return directory
}

func (f erasureFixture) stopReceipt(t *testing.T, run string) {
	t.Helper()
	finished := time.Now().Add(-time.Minute).UTC()
	summary, err := marshalRunHistory(RunDetails{RunSummary: RunSummary{ID: run, ProjectID: f.project, Status: "succeeded", FinishedAt: &finished}, StopConfirmed: true, Nodes: []NodeRun{}})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,$3,$4)`, run, f.ns, finished, summary)
}
