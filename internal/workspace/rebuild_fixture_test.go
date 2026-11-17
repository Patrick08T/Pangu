package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type rebuildFixture struct {
	ctx                                       context.Context
	conn                                      *pgx.Conn
	project                                   *Project
	namespace, directory, source, origin, run string
}

func newRebuildFixture(t *testing.T) rebuildFixture {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	_, file, _, _ := runtime.Caller(0)
	artifacts := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".artifacts"))
	name := "wr-" + uuid.NewString()[:8]
	dir := filepath.Join(artifacts, name)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(dir) != artifacts || filepath.Base(dir) != name || !strings.HasPrefix(name, "wr-") {
			t.Fatal("拒绝清理未核验的测试目录")
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("测试目录已变化", err)
		}
		if err = os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", filepath.Join(dir, "w"))
	t.Setenv("PANGU_WORKER_ID", name)
	f := rebuildFixture{ctx: context.Background(), namespace: name, directory: dir, source: filepath.Join(dir, "s")}
	var err error
	f.conn, err = database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer f.conn.Close(context.Background())
		for _, query := range []string{
			"DELETE FROM pangu.submissions WHERE namespace=$1",
			"DELETE FROM pangu.run_cancellations WHERE namespace=$1",
			"DELETE FROM pangu.workspace_checkpoints WHERE run_id IN (SELECT run_id FROM pangu.workspace_runs WHERE project_id IN (SELECT id FROM pangu.projects WHERE namespace=$1))",
			"DELETE FROM pangu.workspace_attempts WHERE run_id IN (SELECT run_id FROM pangu.workspace_runs WHERE project_id IN (SELECT id FROM pangu.projects WHERE namespace=$1))",
			"DELETE FROM pangu.research_reports WHERE run_id IN (SELECT run_id FROM pangu.workspace_runs WHERE project_id IN (SELECT id FROM pangu.projects WHERE namespace=$1))",
			"DELETE FROM pangu.workspace_runs WHERE project_id IN (SELECT id FROM pangu.projects WHERE namespace=$1)",
			"DELETE FROM pangu.projects WHERE namespace=$1",
		} {
			if _, e := f.conn.Exec(f.ctx, query, f.namespace); e != nil {
				t.Error(e)
			}
		}
	})
	if err = os.Mkdir(f.source, 0700); err != nil {
		t.Fatal(err)
	}
	rebuildGit(t, f.ctx, f.source, "init", "--initial-branch=main")
	rebuildWrite(t, filepath.Join(f.source, "source.txt"), "clean origin")
	rebuildGit(t, f.ctx, f.source, "add", ".")
	rebuildGit(t, f.ctx, f.source, "commit", "-m", "clean origin")
	f.origin = rebuildGit(t, f.ctx, f.source, "rev-parse", "HEAD")
	f.project, err = Register(f.ctx, f.namespace, Project{VCS: "git", Name: name, Source: f.source, Branch: "main", WorkerID: name})
	if err != nil {
		t.Fatal(err)
	}
	f.run = uuid.NewString()
	if err = Ensure(f.ctx, f.run, f.project.ID, f.namespace); err != nil {
		t.Fatal(err)
	}
	f.submission(t, f.run, map[string]any{})
	return f
}

func (f rebuildFixture) submission(t *testing.T, run string, extra map[string]any) {
	t.Helper()
	extra["workflow"] = map[string]string{"project_id": f.project.ID}
	raw, err := json.Marshal(extra)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.conn.Exec(f.ctx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id)
 VALUES($1::uuid,$2,$3,'rebuild-test',($1::uuid)::text,'test',$4,$5)`, uuid.NewString(), f.namespace, f.project.ID, raw, run)
	if err != nil {
		t.Fatal(err)
	}
}

func (f rebuildFixture) fork(t *testing.T) string {
	t.Helper()
	_, err := Execute(f.ctx, f.run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "stale result"}, os.WriteFile(filepath.Join(dir, "polluted.txt"), []byte("revoked influence"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := Get(f.ctx, f.run)
	if err != nil {
		t.Fatal(err)
	}
	child := uuid.NewString()
	f.submission(t, child, map[string]any{"source_run_id": f.run, "fork": map[string]string{"origin_commit": f.origin, "commit": source.HeadCommit}})
	if err = EnsureFork(f.ctx, child, f.project.ID, f.namespace, f.run, source.HeadCommit); err != nil {
		t.Fatal(err)
	}
	return child
}

func rebuildWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func rebuildGit(t *testing.T, ctx context.Context, directory string, args ...string) string {
	t.Helper()
	out, err := git(ctx, directory, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
