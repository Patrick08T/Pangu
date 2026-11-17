package workspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestPerforceOperationUnknownBlocksExistingAndCrossRunEntrypoints(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	namespace := projectNamespace(t, f.project.ID)
	before := largeRecoveryTree(t, f.run.Directory)
	owner, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(owner, errLocalGitStopUnconfirmed)
	_ = finish(nil) // Deliberately lose the outer error: persistence is authoritative.
	checks := map[string]func() error{
		"ready":   func() error { return Ensure(ctx, f.run.ID, f.project.ID, namespace) },
		"source":  func() error { _, err := RerunSource(ctx, f.run.ID, f.project.ID); return err },
		"new-run": func() error { return Ensure(ctx, uuid.NewString(), f.project.ID, namespace) },
		"rerun":   func() error { return EnsureRerun(ctx, uuid.NewString(), f.project.ID, namespace, f.run.ID) },
		"fork": func() error {
			return EnsureFork(ctx, uuid.NewString(), f.project.ID, namespace, f.run.ID, f.run.BaseCommit)
		},
		"delivery": func() error { _, err := DeliverPerforce(ctx, f.run.ID); return err },
		"model": func() error {
			_, err := Execute(ctx, f.run.ID, uuid.NewString(), 1, false, func(string) (workflow.NodeOutput, error) {
				t.Error("unknown Git stop allowed a new model invocation")
				return workflow.NodeOutput{}, nil
			})
			return err
		},
	}
	for name, call := range checks {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("entrypoint bypassed persistent unknown-stop guard")
			}
		})
	}
	if !reflect.DeepEqual(before, largeRecoveryTree(t, f.run.Directory)) {
		t.Fatal("isolated run Git evidence changed")
	}
	var attempts, changes int
	if err := f.conn.QueryRow(ctx, "SELECT count(*) FROM pangu.workspace_attempts WHERE run_id=$1", f.run.ID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := f.conn.QueryRow(ctx, "SELECT count(*) FROM pangu.perforce_sources WHERE run_id=$1 AND work_change IS NOT NULL", f.run.ID).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || changes != 0 {
		t.Fatalf("quarantine allowed attempt/P4 changelist creation: %d/%d", attempts, changes)
	}
}

func TestPerforceOperationUnknownBlocksAfterSnapshotRecovery(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	node := uuid.NewString()
	if _, err := f.conn.Exec(ctx, `INSERT INTO pangu.workspace_attempts(run_id,node_run_id,attempt,directory,base_commit,before_commit,status) VALUES($1,$2,1,$3,$4,$4,'failed')`, f.run.ID, node, f.project.Source, f.run.BaseCommit); err != nil {
		t.Fatal(err)
	}
	owner, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(owner, errLocalGitStopUnconfirmed)
	_ = finish(nil)
	before := largeRecoveryTree(t, f.run.Directory)
	if err := recoverLocalAttempts(ctx, f.conn, f.snapshot); err == nil {
		t.Fatal("recovery crossed persistent unknown-stop guard")
	}
	var after string
	var tracked bool
	if err := f.conn.QueryRow(ctx, "SELECT after_commit,p4_tracked FROM pangu.workspace_attempts WHERE node_run_id=$1", node).Scan(&after, &tracked); err != nil {
		t.Fatal(err)
	}
	if after != "" || tracked || !reflect.DeepEqual(before, largeRecoveryTree(t, f.run.Directory)) {
		t.Fatal("blocked recovery updated immutable evidence or tracking")
	}
}

func TestLocalGitOperationRejectsNewRepositoryAndStaleOwner(t *testing.T) {
	localOperationFixture(t)
	owner, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(owner, errLocalGitStopUnconfirmed)
	_ = finish(nil)
	for _, ctx := range []context.Context{owner, context.Background()} {
		repository := t.TempDir()
		if _, err := runLocalGit(ctx, repository, "init"); err == nil {
			t.Fatal("isolated worker started init for another run")
		}
		if _, err := os.Lstat(filepath.Join(repository, ".git")); !os.IsNotExist(err) {
			t.Fatalf("blocked init changed its target: %v", err)
		}
	}
}
