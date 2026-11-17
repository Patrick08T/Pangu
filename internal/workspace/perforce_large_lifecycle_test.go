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
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestP4LargeSingleRepo(t *testing.T) {
	_, _, id, _ := localP4Run(t)
	ctx := context.Background()
	run, err := Get(ctx, id)
	if err != nil || !run.Ready {
		t.Fatalf("not ready: %+v %v", run, err)
	}
	for _, name := range []string{"source", "base"} {
		if name == "source" {
			if _, err := os.Stat(filepath.Join(run.Directory, name)); !os.IsNotExist(err) {
				t.Errorf("new local P4 run retained redundant source repository: %v", err)
			}
			continue
		}
		base := filepath.Join(run.Directory, name)
		if _, err := os.Stat(filepath.Join(base, ".git")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(base, "edit.txt")); !os.IsNotExist(err) {
			t.Errorf("object repository unexpectedly checked out project files: %v", err)
		}
		body, err := git(ctx, base, "show", run.BaseCommit+":edit.txt")
		if err != nil || body != "old" {
			t.Fatalf("baseline bytes unavailable: %q %v", body, err)
		}
	}
}

func TestP4LargeFailedTracking(t *testing.T) {
	c, _, id, path := localP4Run(t)
	ctx := context.Background()
	p4OK(t, c, "", "edit", "//"+c.Client+"/Server/edit.txt")
	before, err := Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	node := uuid.NewString()
	_, runErr := Execute(ctx, id, node, 1, false, func(dir string) (workflow.NodeOutput, error) {
		if err := writeLargeAttemptFiles(dir); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{}, errors.New("model intentionally failed after writing")
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "model intentionally failed") {
		t.Fatalf("lost model failure: %v", runErr)
	}
	after, err := Get(ctx, id)
	if err != nil || after.HeadCommit != before.HeadCommit {
		t.Fatalf("failed attempt published success: %+v %v", after, err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var checkpoint string
	var tracked bool
	if err := conn.QueryRow(ctx, "SELECT after_commit,p4_tracked FROM pangu.workspace_attempts WHERE node_run_id=$1 AND attempt=1", node).Scan(&checkpoint, &tracked); err != nil {
		t.Fatal(err)
	}
	if checkpoint == "" || !tracked {
		t.Fatalf("large failed attempt not fully captured/tracked: after=%q tracked=%v error=%v", checkpoint, tracked, runErr)
	}
	if _, err := os.Stat(filepath.Join(path, "bulk", "f1000.txt")); err != nil {
		t.Fatal("failed files lost", err)
	}
	s, err := readPerforce(ctx, conn, id)
	if err != nil || s.LocalCommit != checkpoint {
		t.Fatalf("local checkpoint not retained: %+v %v", s, err)
	}
	rows := p4OK(t, c, "", "opened", c.Files(c.Client))
	if len(rows) != 1002 {
		t.Fatalf("incomplete P4 tracking: got %d files, want 1002", len(rows))
	}
	for _, row := range rows {
		want := strconv.FormatInt(s.WorkChange, 10)
		if strings.HasSuffix(row["clientFile"], "/edit.txt") {
			want = "default"
		}
		if row["change"] != want {
			t.Fatalf("changelist ownership changed: %v", row)
		}
	}
	if _, err := Files(ctx, id); err == nil || !strings.Contains(err.Error(), "1000") {
		t.Fatalf("presentation limit changed: %v", err)
	}
}

func writeLargeAttemptFiles(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "bulk"), 0700); err != nil {
		return err
	}
	for i := 0; i < 1001; i++ {
		if err := os.WriteFile(filepath.Join(root, "bulk", fmt.Sprintf("f%04d.txt", i)), []byte("new raw bytes\r\n"), 0600); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(root, "edit.txt"), []byte("existing user change plus attempt\n"), 0600)
}

func TestP4LargeIndependentFork(t *testing.T) {
	_, p, id, _ := localP4Run(t)
	ctx := context.Background()
	prior, err := Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	next := uuid.NewString()
	if err := EnsureFork(ctx, next, p.ID, projectNamespace(t, p.ID), id, prior.HeadCommit); err != nil {
		t.Fatal(err)
	}
	child, err := Get(ctx, next)
	if err != nil || !child.Ready {
		t.Fatalf("fork unavailable: %+v %v", child, err)
	}
	base := filepath.Join(child.Directory, "base")
	if _, err := os.Stat(filepath.Join(base, "edit.txt")); !os.IsNotExist(err) {
		t.Errorf("fork checked out files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		t.Errorf("fork borrows parent object storage: %v", err)
	}
	parentBase := filepath.Join(prior.Directory, "base")
	// Both paths were created by this test, and only this private parent repo is moved.
	if err := os.Rename(parentBase, parentBase+"-offline"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(parentBase+"-offline", parentBase)
	body, err := git(ctx, base, "show", child.HeadCommit+":edit.txt")
	if err != nil || body != "old" {
		t.Fatalf("fork depends on parent: %q %v", body, err)
	}
}
