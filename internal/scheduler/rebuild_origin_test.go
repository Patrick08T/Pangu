package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func TestRebuildForkRestoresCleanOriginInsteadOfPollutedBase(t *testing.T) {
	f := rebuildForkFixture(t)
	old, err := workspace.Get(f.ctx, f.run)
	if err != nil || old.BaseCommit == f.previous.Snapshot.Fork.OriginCommit {
		t.Fatal("fixture did not distinguish fork base from clean origin", old, err)
	}
	child := uuid.NewString()
	next, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) { return child, nil })
	if err != nil {
		t.Fatal("verified clean fork origin was rejected", err)
	}
	if next.Snapshot.Fork != nil || next.Snapshot.ApprovalOrigin != "" {
		t.Fatal("fresh successor retained output/approval reuse")
	}
	if err = prepareWorkspace(f.ctx, child, f.actor.Namespace, runInput{RequestID: next.ID, Snapshot: next.Snapshot}); err != nil {
		t.Fatal(err)
	}
	fresh, err := workspace.Get(f.ctx, child)
	if err != nil || fresh.BaseCommit != f.previous.Snapshot.Fork.OriginCommit {
		t.Fatal("worker did not restore the clean origin", fresh, err)
	}
	if _, err = os.Stat(filepath.Join(fresh.Directory, "base", "polluted.txt")); !os.IsNotExist(err) {
		t.Fatal("revoked checkpoint file survived fresh rebuild", err)
	}
}

func TestRebuildRejectsUnavailableForkSourceBeforePreparing(t *testing.T) {
	for _, column := range []string{"expired", "worker"} {
		t.Run(column, func(t *testing.T) {
			f := rebuildForkFixture(t)
			if column == "expired" {
				rebuildExec(t, `UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1`, f.run)
			} else {
				rebuildExec(t, `UPDATE pangu.workspace_runs SET worker_id='different-worker' WHERE run_id=$1`, f.run)
			}
			item, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) {
				t.Fatal("unavailable clean source reached dispatch")
				return "", nil
			})
			if err == nil || item != nil || rebuildCount(t, f.actor.Namespace) != 0 {
				t.Fatal("invalid source created a successor before worker checkout", item, err)
			}
		})
	}
}

func rebuildForkFixture(t *testing.T) rebuildFixture {
	t.Helper()
	f := newRebuildFixture(t)
	original, err := workspace.Get(f.ctx, f.run)
	if err != nil {
		t.Fatal(err)
	}
	nodeRun := uuid.NewString()
	_, err = workspace.Execute(f.ctx, f.run, nodeRun, 1, false, func(dir string) (workflow.NodeOutput, error) {
		err := os.WriteFile(filepath.Join(dir, "polluted.txt"), []byte("revoked influence"), 0600)
		return workflow.NodeOutput{Text: "stale checkpoint output"}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	polluted, output, err := workspace.Checkpoint(f.ctx, f.run, nodeRun)
	if err != nil {
		t.Fatal(err)
	}
	prior := f.run
	snapshot := submission.Snapshot{Purpose: "business", CreatedBy: f.actor.ID, Workflow: f.previous.Snapshot.Workflow, SourceRunID: prior, Fork: &workflow.Fork{OriginCommit: original.BaseCommit, Commit: polluted, CheckpointNode: "plan", Reused: map[string]workflow.NodeOutput{"plan": output}, Checkpoints: map[string]string{"plan": polluted}}}
	f.previous, err = submission.Prepare(f.ctx, f.actor.Namespace, "fork", "fork-input", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	f.run = uuid.NewString()
	f.previous, err = submission.Dispatch(f.ctx, f.actor.Namespace, f.previous.ID, func(context.Context, *submission.Record) (string, error) { return f.run, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = workspace.EnsureFork(f.ctx, f.run, f.previous.ProjectID, f.actor.Namespace, prior, polluted); err != nil {
		t.Fatal(err)
	}
	f.gate, err = approval.Request(f.ctx, f.actor.Namespace, f.previous.ID, f.run, f.input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = approval.Decide(f.ctx, f.actor, f.gate.ID, approval.Decision{SubjectHash: f.gate.SubjectHash, Decision: "approved", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}}}); err != nil {
		t.Fatal(err)
	}
	f.intent, err = approval.Invalidate(f.ctx, f.actor.Namespace, f.run, f.input.Node.ID, 1, "revoked checkpoint dependency")
	if err != nil {
		t.Fatal(err)
	}
	return f
}
