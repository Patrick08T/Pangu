package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type largeCancelFixture struct {
	config      perforce.Config
	conn        *pgx.Conn
	run         string
	source      string
	path        string
	head        string
	successNode string
	node        string
	markerInfo  os.FileInfo
	markerBody  string
}

type largeCancelState struct {
	before, after, local, head, status string
	tracked                            bool
	work                               int64
	paths, checkpoints                 int
}

// Removing recovery, reusing the cancelled context for finish, truncating the
// internal path list, or admitting a callback before tracking must fail here.
func TestP4LargeCancelledCaptureRecoversBeforeNextCallback(t *testing.T) {
	f := newLargeCancelFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := Execute(ctx, f.run, f.node, 1, false, func(dir string) (workflow.NodeOutput, error) {
		if dir != f.path {
			t.Fatalf("callback left test-owned P4 scope: %q", dir)
		}
		if err := writeLargeAttemptFiles(dir); err != nil {
			return workflow.NodeOutput{}, err
		}
		f.createMarker(t)
		cancel()
		return workflow.NodeOutput{}, context.Canceled
	})
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "根目录") || !strings.Contains(err.Error(), "Git 元数据") {
		t.Fatalf("cancelled attempt did not reach detached strict after-capture: %v", err)
	}
	f.assertPending(t)
	f.assertFiles(t)
	f.removeMarker(t)
	next := uuid.NewString()
	f.assertStopGuard(t, next)
	// The synchronous callback has returned and spawned no process. Record the
	// scheduler's stop confirmation only after proving its unfinished row blocks.
	f.execOwnAttempt(t, `UPDATE pangu.node_attempts SET finished_at=now(),status='cancelled'
		WHERE run_id=$1 AND node_run_id=$2 AND attempt=1 AND finished_at IS NULL`)
	called := false
	_, err = Execute(context.Background(), f.run, next, 1, true, func(dir string) (workflow.NodeOutput, error) {
		called = true
		if dir != f.path {
			t.Fatalf("recovery changed the bound scope: %q", dir)
		}
		f.assertRecovered(t)
		f.assertFiles(t)
		return workflow.NodeOutput{}, nil
	})
	if err != nil || !called {
		t.Fatalf("read-only next callback was not admitted after recovery: called=%v error=%v", called, err)
	}
	f.assertPublishedNext(t, next)
}

func newLargeCancelFixture(t *testing.T) *largeCancelFixture {
	t.Helper()
	// Standard isolated-DSN/P4 skip and generated client, worker, run and TempDirs.
	c, project, id, path := localP4Run(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	f := &largeCancelFixture{config: c, conn: conn, run: id, source: project.Source,
		path: path, successNode: uuid.NewString(), node: uuid.NewString()}
	f.privateMarkerPath(t)
	_, err = Execute(ctx, id, f.successNode, 1, true, func(string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := Get(ctx, id)
	if err != nil || !run.Ready || run.HeadCommit == "" || run.HeadCommit == run.BaseCommit {
		t.Fatalf("fixture lacks a distinct successful head: %+v %v", run, err)
	}
	f.head = run.HeadCommit
	p4OK(t, c, "", "edit", "//"+c.Client+"/Server/edit.txt")
	f.execOwnAttempt(t, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt)
		VALUES($1,$2,'large-cancel',1)`)
	return f
}

func (f *largeCancelFixture) execOwnAttempt(t *testing.T, query string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" || os.Getenv("PANGU_DATABASE_URL") != os.Getenv("PANGU_TEST_DATABASE_URL") {
		t.Fatal("attempt fixture requires the current isolated test DSN")
	}
	tag, err := f.conn.Exec(context.Background(), query, f.run, f.node)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("test-owned attempt mutation affected %d rows: %v", tag.RowsAffected(), err)
	}
}

func (f *largeCancelFixture) state(t *testing.T) largeCancelState {
	t.Helper()
	var s largeCancelState
	err := f.conn.QueryRow(context.Background(), `SELECT a.before_commit,a.after_commit,a.p4_tracked,a.status,
		s.local_commit,s.work_change,jsonb_array_length(s.tracked_paths),w.head_commit,
		(SELECT count(*) FROM pangu.workspace_checkpoints c WHERE c.run_id=a.run_id)
		FROM pangu.workspace_attempts a JOIN pangu.perforce_sources s USING(run_id)
		JOIN pangu.workspace_runs w USING(run_id)
		WHERE a.run_id=$1 AND a.node_run_id=$2 AND a.attempt=1`, f.run, f.node).
		Scan(&s.before, &s.after, &s.tracked, &s.status, &s.local, &s.work, &s.paths, &s.head, &s.checkpoints)
	if err != nil {
		t.Fatal(err)
	}
	if s.before == "" || s.status != "failed" || s.head != f.head || s.work <= 0 || s.checkpoints != 1 {
		t.Fatalf("failed attempt changed successful state: %+v; successful head=%s", s, f.head)
	}
	f.assertCheckpoints(t)
	return s
}

func (f *largeCancelFixture) assertCheckpoints(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	var count int
	if err := f.conn.QueryRow(ctx, `SELECT count(*) FROM pangu.workspace_checkpoints
		WHERE run_id=$1 AND node_run_id=$2`, f.run, f.node).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cancelled attempt published a checkpoint: count=%d error=%v", count, err)
	}
	commit, _, err := Checkpoint(ctx, f.run, f.successNode)
	if err != nil || commit != f.head {
		t.Fatalf("previous successful checkpoint changed: %q %v; want %s", commit, err, f.head)
	}
}

func (f *largeCancelFixture) assertPending(t *testing.T) {
	t.Helper()
	s := f.state(t)
	if s.after != "" || s.tracked || s.local != f.head || s.paths != 0 {
		t.Fatalf("failed after-capture was not left pending: %+v", s)
	}
	rows := p4OK(t, f.config, "", "opened", f.config.Files(f.config.Client))
	if len(rows) != 1 || rows[0]["clientFile"] != "//"+f.config.Client+"/Server/edit.txt" || rows[0]["change"] != "default" || rows[0]["action"] != "edit" {
		t.Fatalf("failed capture altered existing ownership or partially tracked new files: %v", rows)
	}
}

func (f *largeCancelFixture) assertStopGuard(t *testing.T, next string) {
	t.Helper()
	called := false
	_, err := Execute(context.Background(), f.run, next, 1, true, func(string) (workflow.NodeOutput, error) {
		called = true
		return workflow.NodeOutput{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "尚未确认停止") || called {
		t.Fatalf("unfinished process-stop record did not block callback: called=%v error=%v", called, err)
	}
	var count int
	if err := f.conn.QueryRow(context.Background(), `SELECT count(*) FROM pangu.workspace_attempts
		WHERE run_id=$1 AND node_run_id=$2`, f.run, next).Scan(&count); err != nil || count != 0 {
		t.Fatalf("blocked callback acquired a workspace attempt: count=%d error=%v", count, err)
	}
	f.assertPending(t)
}

func (f *largeCancelFixture) assertRecovered(t *testing.T) {
	t.Helper()
	s := f.state(t)
	// 1001 new files plus the one already-open edit; no UI 1000-path cap.
	if s.after == "" || s.after == s.before || s.after == f.head || !s.tracked || s.local != s.after || s.paths != 1002 {
		t.Fatalf("next callback started before complete recovery: %+v", s)
	}
	rows := p4OK(t, f.config, "", "opened", f.config.Files(f.config.Client))
	if len(rows) != 1002 {
		t.Fatalf("incomplete recovered P4 ownership: got %d, want 1002", len(rows))
	}
	opened := make(map[string]perforce.Record, len(rows))
	for _, row := range rows {
		name := row["clientFile"]
		if _, duplicate := opened[name]; duplicate {
			t.Fatalf("duplicate opened path: %s", name)
		}
		opened[name] = row
	}
	prefix := "//" + f.config.Client + "/Server/"
	if row := opened[prefix+"edit.txt"]; row["change"] != "default" || row["action"] != "edit" {
		t.Fatalf("original default changelist ownership changed: %v", row)
	}
	for i := 0; i < 1001; i++ {
		name := prefix + fmt.Sprintf("bulk/f%04d.txt", i)
		row := opened[name]
		if row["change"] != strconv.FormatInt(s.work, 10) || row["action"] != "add" {
			t.Fatalf("new path %s not owned by WorkChange %d: %v", name, s.work, row)
		}
	}
}

func (f *largeCancelFixture) assertFiles(t *testing.T) {
	t.Helper()
	for i := 0; i < 1001; i++ {
		name := filepath.Join(f.path, "bulk", fmt.Sprintf("f%04d.txt", i))
		body, err := os.ReadFile(name)
		if err != nil || string(body) != "new raw bytes\r\n" {
			t.Fatalf("cancel/recovery lost file %d bytes: %q %v", i, body, err)
		}
	}
	body, err := os.ReadFile(filepath.Join(f.path, "edit.txt"))
	if err != nil || string(body) != "existing user change plus attempt\n" {
		t.Fatalf("cancel/recovery reverted the already-open edit: %q %v", body, err)
	}
}

func (f *largeCancelFixture) assertPublishedNext(t *testing.T, next string) {
	t.Helper()
	ctx := context.Background()
	commit, _, err := Checkpoint(ctx, f.run, next)
	if err != nil || commit == "" || commit == f.head {
		t.Fatalf("read-only no-op could not finish normally: %q %v", commit, err)
	}
	run, err := Get(ctx, f.run)
	if err != nil || run.HeadCommit != commit {
		t.Fatalf("next success did not publish its own head: %+v %v", run, err)
	}
	var count int
	if err := f.conn.QueryRow(ctx, `SELECT count(*) FROM pangu.workspace_checkpoints
		WHERE run_id=$1`, f.run).Scan(&count); err != nil || count != 2 {
		t.Fatalf("expected only initial and next successful checkpoints: count=%d error=%v", count, err)
	}
	f.assertCheckpoints(t)
}

func (f *largeCancelFixture) privateMarkerPath(t *testing.T) string {
	t.Helper()
	// Both roots came from this localP4Run's own p4Server TempDir, not user P4.
	if f.config.Scope != "Server" || f.path != filepath.Join(f.source, "Server") {
		t.Fatal("marker is not in this fixture's bound P4 scope")
	}
	if _, err := uuid.Parse(strings.TrimPrefix(f.config.Client, "test-")); err != nil || !strings.HasPrefix(f.config.Client, "test-") {
		t.Fatal("marker fixture does not own a generated P4 client")
	}
	for _, root := range []string{f.source, f.path} {
		actual, err := filepath.EvalSymlinks(root)
		if err != nil || !filepath.IsAbs(root) || filepath.Clean(root) != root {
			t.Fatalf("private fixture root is not canonical: %q %v", root, err)
		}
		rel, err := filepath.Rel(root, actual)
		if err != nil || rel != "." {
			t.Fatalf("private fixture root was redirected: %q -> %q %v", root, actual, err)
		}
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("private fixture root is not a real directory: %q %v", root, err)
		}
	}
	marker := filepath.Join(f.path, ".git")
	if rel, err := filepath.Rel(f.source, marker); err != nil || rel != filepath.Join("Server", ".git") {
		t.Fatalf("marker escaped the exact private scope: %q %v", marker, err)
	}
	return marker
}

func (f *largeCancelFixture) createMarker(t *testing.T) {
	t.Helper()
	path := f.privateMarkerPath(t)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatalf("refuse to replace any existing root .git: %v", err)
	}
	f.markerBody = "pangu test-owned cancelled-capture marker " + f.node + "\n"
	_, writeErr := file.WriteString(f.markerBody)
	info, statErr := file.Stat()
	closeErr := file.Close()
	if err := errors.Join(writeErr, statErr, closeErr); err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatal("created marker is not a regular file")
	}
	f.markerInfo = info
}

func (f *largeCancelFixture) removeMarker(t *testing.T) {
	t.Helper()
	path := f.privateMarkerPath(t)
	info, err := os.Lstat(path)
	if err != nil || f.markerInfo == nil || !info.Mode().IsRegular() || !os.SameFile(info, f.markerInfo) {
		t.Fatalf("refuse to remove anything but the exact created root .git file: %v", err)
	}
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(path, actual); err != nil || rel != "." {
		t.Fatalf("refuse to remove redirected root .git: %q %v", actual, err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != f.markerBody {
		t.Fatalf("refuse to remove root .git with unknown content: %v", err)
	}
	// No recursive deletion: only this exact exclusive-created regular marker.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("test-owned root marker still present: %v", err)
	}
}
