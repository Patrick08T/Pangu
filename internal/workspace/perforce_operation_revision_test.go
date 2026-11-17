package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestPerforceOperationContentRevisionRejectsUnknownGitStop(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	if tree, err := ContentRevision(ctx, f.run.ID, f.run.BaseCommit); err != nil || !commitID.MatchString(tree) {
		t.Fatalf("healthy content revision rejected: %q %v", tree, err)
	}
	owner, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(owner, errLocalGitStopUnconfirmed)
	_ = finish(nil)
	if tree, err := ContentRevision(ctx, f.run.ID, f.run.BaseCommit); err == nil || tree != "" {
		t.Fatalf("content revision Git bypassed quarantine: %q %v", tree, err)
	}
}

func TestPerforceOperationRevisionQueriesInsideModel(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	node := uuid.NewString()
	called := false
	_, err := Execute(ctx, f.run.ID, node, 1, true, func(path string) (workflow.NodeOutput, error) {
		called = true
		input := workflow.NodeInput{RunID: f.run.ID, ProjectID: f.project.ID, NodeRunID: node, Workspace: path}
		r, err := RevisionFor(ctx, input)
		if err != nil || r == nil || !r.LocalPerforce {
			t.Fatalf("healthy callback revision rejected: %+v %v", r, err)
		}
		if tree, err := ContentRevision(ctx, f.run.ID, r.HeadCommit); err != nil || !commitID.MatchString(tree) {
			t.Fatalf("healthy callback content revision rejected: %q %v", tree, err)
		}
		if _, err := Files(ctx, f.run.ID); err != nil {
			t.Fatalf("healthy callback artifact query rejected: %v", err)
		}
		if _, _, err := Document(ctx, f.run.ID, "edit.txt"); err != nil {
			t.Fatalf("healthy callback document query rejected: %v", err)
		}
		if err := checkLocalGitOperation(ctx); err != nil {
			t.Fatalf("completed callback query left Git quarantine: %v", err)
		}
		return workflow.NodeOutput{}, workflow.ErrStopUnconfirmed
	})
	if !called || !errors.Is(err, workflow.ErrStopUnconfirmed) {
		t.Fatalf("callback failed or lost its stop state: %v %v", called, err)
	}
}

func TestPerforceOperationRevisionRejectsUnknownGitStop(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	path, err := f.snapshot.LocalDirectory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	node := uuid.NewString()
	if _, err := f.conn.Exec(ctx, `INSERT INTO pangu.workspace_attempts(run_id,node_run_id,attempt,directory,base_commit,before_commit,status) VALUES($1,$2,1,$3,$4,$4,'running')`, f.run.ID, node, path, f.run.BaseCommit); err != nil {
		t.Fatal(err)
	}
	input := workflow.NodeInput{RunID: f.run.ID, ProjectID: f.project.ID, NodeRunID: node, Workspace: path}
	if revision, local, err := localRevision(ctx, f.conn, input); err != nil || !local || revision == nil {
		t.Fatalf("healthy local revision fixture rejected: %v %v", local, err)
	}
	owner, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(owner, errLocalGitStopUnconfirmed)
	_ = finish(nil)
	if _, local, err := localRevision(ctx, f.conn, input); !local || err == nil {
		t.Fatalf("revision Git bypassed quarantine: local=%v error=%v", local, err)
	}
}
