package workspace

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type largeRecoveryFixture struct {
	project  *Project
	run      *Run
	snapshot *perforce.Snapshot
	conn     *pgx.Conn
}

func newLargeRecoveryFixture(t *testing.T) largeRecoveryFixture {
	t.Helper()
	// localP4Run supplies the standard database skip, test DSN and private roots.
	_, project, id, _ := localP4Run(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	run, err := readRun(ctx, conn, id)
	if err != nil || !run.Ready {
		t.Fatalf("fixture run unavailable: %+v %v", run, err)
	}
	snapshot, err := readPerforce(ctx, conn, id)
	if err != nil {
		t.Fatal(err)
	}
	return largeRecoveryFixture{project: project, run: run, snapshot: snapshot, conn: conn}
}

func TestP4LargePublicationRecoversBeforeDatabaseCommit(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	base := largeRecoveryRunPath(t, f.run, "base")
	repositoryBefore := largeRecoveryTree(t, base)
	workspaceBefore := largeRecoveryTree(t, f.project.Source)
	// Recreate the crash boundary after staging -> base, before either commit
	// registration or workspace_runs insertion. Only this helper-created ID is changed.
	largeRecoverySQL(t, f.conn, "DELETE FROM pangu.workspace_runs WHERE run_id=$1", f.run.ID)
	largeRecoverySQL(t, f.conn, "UPDATE pangu.perforce_sources SET commit_id='',local_commit='' WHERE run_id=$1", f.run.ID)
	if err := Ensure(ctx, f.run.ID, f.project.ID, projectNamespace(t, f.project.ID)); err != nil {
		t.Fatal(err)
	}
	largeRecoveryAssertReady(t, f, f.run.BaseCommit, f.run.HeadCommit)
	snapshot, err := readPerforce(ctx, f.conn, f.run.ID)
	if err != nil || snapshot.Commit != f.run.BaseCommit || snapshot.LocalCommit != f.run.BaseCommit {
		t.Fatalf("published checkpoint not registered: %+v %v", snapshot, err)
	}
	if !reflect.DeepEqual(repositoryBefore, largeRecoveryTree(t, base)) {
		t.Fatal("recovery rewrote the already published repository")
	}
	if !reflect.DeepEqual(workspaceBefore, largeRecoveryTree(t, f.project.Source)) {
		t.Fatal("publication recovery modified the P4 workspace")
	}
	largeRecoveryAssertEmptyWorktree(t, base)
	if _, err := os.Lstat(largeRecoveryRunPath(t, f.run, "source")); !os.IsNotExist(err) {
		t.Fatalf("recovery recreated a redundant source: %v", err)
	}
}

func TestP4LargePublicationRejectsIncompleteEvidence(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	cases := []string{"missing-marker", "malformed-marker", "wrong-run", "wrong-head", "missing-commit", "corrupt-commit", "missing-blob", "corrupt-blob", "hash-mismatched-blob"}
	for _, mode := range cases {
		t.Run(mode, func(t *testing.T) {
			base, commit := largeRecoveryPublishedFixture(t, f.run.ID)
			snapshot := *f.snapshot
			snapshot.Commit, snapshot.LocalCommit = "", ""
			ctx := context.Background()
			if got, err := readPublishedLocalCapture(ctx, &snapshot, base); err != nil || got != commit {
				t.Fatalf("undamaged publication rejected: %q %v", got, err)
			}
			largeRecoveryDamagePublication(t, base, f.run.ID, commit, mode)
			before := largeRecoveryTree(t, base)
			got, err := prepareLocalObjectRepository(ctx, &snapshot, filepath.Dir(base), "", f.project.ID)
			if err == nil || got != "" {
				t.Fatalf("%s evidence accepted: commit=%q error=%v", mode, got, err)
			}
			// Includes HEAD and refs even when the referenced commit is damaged.
			if !reflect.DeepEqual(before, largeRecoveryTree(t, base)) {
				t.Fatal("failed recovery modified publication evidence or HEAD")
			}
		})
	}
}

func largeRecoveryPublishedFixture(t *testing.T, id string) (string, string) {
	t.Helper()
	base := filepath.Join(t.TempDir(), "base")
	if err := os.Mkdir(base, 0700); err != nil {
		t.Fatal(err)
	}
	largeRecoveryGit(t, base, "init", "--initial-branch=main")
	largeRecoveryGit(t, base, "commit", "--allow-empty", "-m", "fixture root")
	// Ordinary Git creates loose objects, so each damaged object is unambiguous.
	localTestWrite(t, base, "edit.txt", []byte("old\n"))
	largeRecoveryGit(t, base, "-c", "core.autocrlf=false", "add", "--", "edit.txt")
	largeRecoveryGit(t, base, "commit", "-m", "published fixture")
	commit := largeRecoveryGit(t, base, "rev-parse", "HEAD")
	largeRecoveryWriteMarker(t, base, localCaptureMarker{RunID: id, Commit: commit})
	// This is the fixture's private checkout, never the bound P4 workspace.
	path := largeRecoveryPrivatePath(t, base, "edit.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	return base, commit
}

func largeRecoveryDamagePublication(t *testing.T, base, id, commit, mode string) {
	t.Helper()
	marker := largeRecoveryPrivatePath(t, base, ".git", "pangu-local-capture.json")
	switch mode {
	case "missing-marker":
		// base is a fresh TempDir fixture; only its verified marker is removed.
		if err := os.Remove(marker); err != nil {
			t.Fatal(err)
		}
	case "malformed-marker":
		localTestWrite(t, base, ".git/pangu-local-capture.json", []byte("{incomplete"))
	case "wrong-run":
		largeRecoveryWriteMarker(t, base, localCaptureMarker{RunID: uuid.NewString(), Commit: commit})
	case "wrong-head":
		parent := largeRecoveryGit(t, base, "rev-parse", "HEAD^")
		largeRecoveryWriteMarker(t, base, localCaptureMarker{RunID: id, Commit: parent})
	default:
		oid := commit
		if strings.HasSuffix(mode, "blob") {
			oid = largeRecoveryGit(t, base, "rev-parse", commit+":edit.txt")
		}
		largeRecoveryDamageObject(t, base, oid, mode)
	}
}

func largeRecoveryDamageObject(t *testing.T, base, oid, mode string) {
	t.Helper()
	if !commitID.MatchString(oid) {
		t.Fatalf("invalid fixture object ID %q", oid)
	}
	path := largeRecoveryPrivatePath(t, base, ".git", "objects", oid[:2], oid[2:])
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("expected fixture loose object: %v", err)
	}
	// Only this newly created TempDir's verified loose object may be damaged.
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(mode, "missing-") {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	data := []byte("not a zlib object")
	if mode == "hash-mismatched-blob" {
		// A valid header and equal-size body defeat size-only ls-tree validation.
		var compressed bytes.Buffer
		writer := zlib.NewWriter(&compressed)
		if _, err := writer.Write([]byte("blob 4\x00bad\n")); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		data = compressed.Bytes()
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func largeRecoveryWriteMarker(t *testing.T, base string, marker localCaptureMarker) {
	t.Helper()
	data, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	localTestWrite(t, base, ".git/pangu-local-capture.json", data)
}

func TestP4LargeHistoricalLayoutsRecover(t *testing.T) {
	for _, layout := range []string{"source-only", "source-and-base"} {
		t.Run(layout, func(t *testing.T) {
			f := newLargeRecoveryFixture(t)
			base := largeRecoveryRunPath(t, f.run, "base")
			source := largeRecoveryRunPath(t, f.run, "source")
			if layout == "source-only" {
				// Both paths are verified children of this localP4Run's private root.
				if err := os.Rename(base, source); err != nil {
					t.Fatal(err)
				}
			} else {
				largeRecoveryGit(t, f.run.Directory, "clone", "--no-local", "--no-hardlinks", "--", base, source)
				largeRecoveryGit(t, base, "checkout", "HEAD", "--", ".")
			}
			largeRecoveryGit(t, source, "checkout", "HEAD", "--", ".")
			sourceBefore := largeRecoveryTree(t, source)
			workspaceBefore := largeRecoveryTree(t, f.project.Source)
			var baseBefore map[string]string
			if layout == "source-and-base" {
				baseBefore = largeRecoveryTree(t, base)
			}
			largeRecoverySQL(t, f.conn, "UPDATE pangu.workspace_runs SET ready=false WHERE run_id=$1", f.run.ID)
			if err := Ensure(context.Background(), f.run.ID, f.project.ID, projectNamespace(t, f.project.ID)); err != nil {
				t.Fatal(err)
			}
			largeRecoveryAssertReady(t, f, f.run.BaseCommit, f.run.HeadCommit)
			if !reflect.DeepEqual(sourceBefore, largeRecoveryTree(t, source)) {
				t.Fatal("historical source was modified")
			}
			if !reflect.DeepEqual(workspaceBefore, largeRecoveryTree(t, f.project.Source)) {
				t.Fatal("historical recovery modified the P4 workspace")
			}
			if layout == "source-only" {
				largeRecoveryAssertEmptyWorktree(t, base)
			} else if !reflect.DeepEqual(baseBefore, largeRecoveryTree(t, base)) {
				t.Fatal("existing historical base was rewritten")
			}
			largeRecoveryAssertIndependent(t, f.run, source, base)
		})
	}
}

func TestP4LargeForkRecoversAfterHeadCASBeforeReady(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	node := uuid.NewString()
	// Publish a real success checkpoint distinct from the parent's immutable base,
	// without changing any P4 files or duplicating the large-path lifecycle test.
	_, err := Execute(ctx, f.run.ID, node, 1, true, func(string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := readRun(ctx, f.conn, f.run.ID)
	if err != nil || parent.HeadCommit == parent.BaseCommit {
		t.Fatalf("distinct success checkpoint missing: %+v %v", parent, err)
	}
	childID, namespace := uuid.NewString(), projectNamespace(t, f.project.ID)
	if err := EnsureFork(ctx, childID, f.project.ID, namespace, parent.ID, parent.HeadCommit); err != nil {
		t.Fatal(err)
	}
	child, err := readRun(ctx, f.conn, childID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := readPerforce(ctx, f.conn, childID)
	if err != nil || snapshot.Commit == child.BaseCommit {
		t.Fatalf("fork did not retain its separate equal-tree capture: %+v %v", snapshot, err)
	}
	base := largeRecoveryRunPath(t, child, "base")
	if head := largeRecoveryGit(t, base, "rev-parse", "HEAD"); head != parent.HeadCommit {
		t.Fatalf("fork CAS has not selected the checkpoint: %s", head)
	}
	before := largeRecoveryTree(t, base)
	// Rewind only this test-created child's final ready write: its actual Git HEAD
	// is already the CAS result, while perforce_sources still names the capture.
	largeRecoverySQL(t, f.conn, "UPDATE pangu.workspace_runs SET ready=false WHERE run_id=$1", childID)
	if err := EnsureFork(ctx, childID, f.project.ID, namespace, parent.ID, parent.HeadCommit); err != nil {
		t.Fatal(err)
	}
	childFixture := largeRecoveryFixture{run: child, conn: f.conn}
	largeRecoveryAssertReady(t, childFixture, parent.HeadCommit, parent.HeadCommit)
	largeRecoveryAssertReady(t, f, f.run.BaseCommit, parent.HeadCommit)
	after, err := readPerforce(ctx, f.conn, childID)
	if err != nil || !reflect.DeepEqual(snapshot, after) {
		t.Fatalf("recovery rewrote capture identity: %+v %v", after, err)
	}
	checkpoint, _, err := Checkpoint(ctx, parent.ID, node)
	if err != nil || checkpoint != parent.HeadCommit {
		t.Fatalf("parent success checkpoint changed: %q %v", checkpoint, err)
	}
	if !reflect.DeepEqual(before, largeRecoveryTree(t, base)) {
		t.Fatal("recovery rewrote the already completed fork HEAD CAS")
	}
	largeRecoveryAssertEmptyWorktree(t, base)
}

func largeRecoveryAssertReady(t *testing.T, f largeRecoveryFixture, base, head string) {
	t.Helper()
	run, err := readRun(context.Background(), f.conn, f.run.ID)
	if err != nil || !run.Ready || run.BaseCommit != base || run.HeadCommit != head {
		t.Fatalf("ready/immutable checkpoint mismatch: %+v %v; want base=%s head=%s", run, err, base, head)
	}
	repository := largeRecoveryRunPath(t, run, "base")
	// Successful attempts advance the DB head, not this immutable repository HEAD.
	if actual := largeRecoveryGit(t, repository, "rev-parse", "HEAD"); actual != base {
		t.Fatalf("Git HEAD=%s, immutable base=%s", actual, base)
	}
	if body := largeRecoveryGit(t, repository, "show", base+":edit.txt"); body != "old" {
		t.Fatalf("baseline content unavailable: %q", body)
	}
}

func largeRecoveryAssertEmptyWorktree(t *testing.T, base string) {
	t.Helper()
	entries, err := os.ReadDir(base)
	if err != nil || len(entries) != 1 || entries[0].Name() != ".git" || !entries[0].IsDir() {
		t.Fatalf("expected only .git in no-checkout base: %v %v", entries, err)
	}
	if _, err := os.Lstat(filepath.Join(base, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		t.Fatalf("base borrows object storage: %v", err)
	}
}

func largeRecoveryAssertIndependent(t *testing.T, run *Run, source, base string) {
	t.Helper()
	for name := range largeRecoveryTree(t, filepath.Join(source, ".git", "objects")) {
		left, err := os.Stat(filepath.Join(source, ".git", "objects", name))
		if err != nil {
			t.Fatal(err)
		}
		right, err := os.Stat(filepath.Join(base, ".git", "objects", name))
		if err == nil && !left.IsDir() && os.SameFile(left, right) {
			t.Fatalf("base shares a hardlinked source object: %s", name)
		}
	}
	offline := largeRecoveryRunPath(t, run, "source-offline")
	// Only the verified private historical repository is moved, never project.Source.
	if err := os.Rename(source, offline); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(offline, source); err != nil {
			t.Errorf("restore private historical repository: %v", err)
		}
	}()
	if body := largeRecoveryGit(t, base, "show", run.BaseCommit+":edit.txt"); body != "old" {
		t.Fatalf("base depends on historical source: %q", body)
	}
}

func largeRecoveryRunPath(t *testing.T, run *Run, name string) string {
	t.Helper()
	root := os.Getenv("PANGU_WORKSPACE_ROOT")
	if _, err := uuid.Parse(run.ID); err != nil || root == "" || run.Directory != filepath.Join(root, run.ID) {
		t.Fatal("path is not derived from the current test-created run")
	}
	return largeRecoveryPrivatePath(t, run.Directory, name)
}

func largeRecoveryPrivatePath(t *testing.T, root string, parts ...string) string {
	t.Helper()
	resolved, err := existingPath(root)
	if err != nil || !filepath.IsAbs(root) || !samePath(resolved, root) {
		t.Fatalf("private fixture root is not canonical: %q %v", root, err)
	}
	target := filepath.Join(append([]string{root}, parts...)...)
	relative, err := filepath.Rel(root, target)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		t.Fatalf("fixture operation escaped private root: %q %v", target, err)
	}
	actual, err := canonicalTarget(target)
	if err != nil || !samePath(actual, target) {
		t.Fatalf("fixture target was redirected: %q %v", target, err)
	}
	return target
}

func largeRecoveryTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unexpected non-regular fixture file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[name] = fmt.Sprintf("%v:%d:%x", info.Mode(), info.ModTime().UnixNano(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func largeRecoveryGit(t *testing.T, repository string, args ...string) string {
	t.Helper()
	out, err := git(context.Background(), repository, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func largeRecoverySQL(t *testing.T, conn *pgx.Conn, query, id string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" || os.Getenv("PANGU_DATABASE_URL") != os.Getenv("PANGU_TEST_DATABASE_URL") {
		t.Fatal("recovery fixture requires the current isolated test DSN")
	}
	tag, err := conn.Exec(context.Background(), query, id)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("test-created run update affected %d rows: %v", tag.RowsAffected(), err)
	}
}
