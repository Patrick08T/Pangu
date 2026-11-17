package scheduler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

type experienceForkFixture struct {
	ctx      context.Context
	ns, run  string
	source   *nodeExperience
	previous *submission.Record
	snapshot submission.Snapshot
	memories []workflow.Experience
}

func newExperienceForkFixture(t *testing.T) experienceForkFixture {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要隔离测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	root := experienceForkDirectory(t)
	t.Setenv("PANGU_WORKSPACE_ROOT", filepath.Join(root, "w"))
	t.Setenv("PANGU_WORKER_ID", "fork-proof")
	dir := filepath.Join(root, "s")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "test"}, {"config", "user.email", "test@example.com"}, {"commit", "--allow-empty", "-m", "origin"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	actor := identity.Actor{ID: uuid.NewString(), Namespace: "fork-proof-" + uuid.NewString()}
	ctx := identity.WithActor(context.Background(), actor)
	project, err := workspace.Register(ctx, actor.Namespace, workspace.Project{VCS: "git", Name: "fork proof", Source: dir, Branch: "main", WorkerID: "fork-proof"})
	if err != nil {
		t.Fatal(err)
	}
	rebuildExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, actor.ID, actor.Namespace)
	rebuildExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role,enabled) VALUES($1,$2,'operator',true)`, actor.ID, project.ID)
	d := workflow.Definition{ProjectID: project.ID, Input: "task", Nodes: []workflow.Node{{ID: "first", Executor: "codex", Instruction: "task"}, {ID: "second", Executor: "codex", Instruction: "task", DependsOn: []string{"first"}}, {ID: "last", Executor: "eino", Model: "fixture", Instruction: "task", DependsOn: []string{"second"}}}}
	previous, err := submission.Prepare(ctx, actor.Namespace, "submit", "original", submission.Snapshot{Workflow: d})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	previous, err = submission.Dispatch(ctx, actor.Namespace, previous.ID, func(context.Context, *submission.Record) (string, error) { return run, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = workspace.Ensure(ctx, run, project.ID, actor.Namespace); err != nil {
		t.Fatal(err)
	}
	source, err := loadNodeExperience(ctx, actor.Namespace, previous.ID, run, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Get(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	fork := &workflow.Fork{CheckpointNode: "second", OriginCommit: ws.BaseCommit, Commit: ws.BaseCommit, Reused: map[string]workflow.NodeOutput{}, Checkpoints: map[string]string{}, ReportNodes: map[string]bool{}, ReportHashes: map[string]string{}}
	return experienceForkFixture{ctx, actor.Namespace, run, source, previous, submission.Snapshot{Workflow: d, SourceRunID: run, Fork: fork}, nil}
}

func experienceForkDirectory(t *testing.T) string {
	t.Helper()
	base, err := filepath.Abs(filepath.Join("..", "..", ".artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := os.MkdirTemp(base, "ef-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(path) != base {
			t.Fatal("unsafe cleanup")
		}
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	})
	return path
}

func (f *experienceForkFixture) execute(t *testing.T, index int) {
	t.Helper()
	d := f.previous.Snapshot.Workflow
	input := d.NodeInput(d.Nodes[index], f.snapshot.Fork.Reused)
	if err := f.source.Prepare(f.ctx, &input); err != nil {
		t.Fatal(err)
	}
	if index == 0 && len(f.memories) != 0 {
		input.Experiences = f.memories
	}
	input.RunID, input.NodeRunID = f.run, uuid.NewString()
	_, err := workspace.Execute(f.ctx, f.run, input.NodeRunID, 1, false, func(dir string) (workflow.NodeOutput, error) {
		if err := os.WriteFile(filepath.Join(dir, input.Node.ID+".txt"), []byte("actual patch"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		report := workflow.Report{ID: uuid.NewString(), Title: "source report", Markdown: "verified report for " + input.Node.ID}
		out := workflow.NodeOutput{Text: input.Node.ID + " output", RunID: input.NodeRunID, ReportIDs: []string{report.ID}, Reports: []workflow.Report{report}, ReportSHA256: workflow.ReportHash(report.Markdown)}
		err := f.source.Record(f.ctx, input, input, &out)
		return out, err
	})
	if err != nil {
		t.Fatal(err)
	}
	commit, out, err := workspace.Checkpoint(f.ctx, f.run, input.NodeRunID)
	if err != nil {
		t.Fatal(err)
	}
	f.snapshot.Fork.Reused[input.Node.ID] = out
	f.snapshot.Fork.Checkpoints[input.Node.ID] = commit
	f.snapshot.Fork.Commit = commit
}

func (f experienceForkFixture) target(t *testing.T) *nodeExperience {
	t.Helper()
	request, err := submission.Prepare(f.ctx, f.ns, "fork", uuid.NewString(), f.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	request, err = submission.Dispatch(f.ctx, f.ns, request.ID, func(context.Context, *submission.Record) (string, error) { return run, nil })
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := loadNodeExperience(f.ctx, f.ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}
