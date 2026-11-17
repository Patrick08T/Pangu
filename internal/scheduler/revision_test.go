package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func revisionRun(t *testing.T) (string, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "revision-worker")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "测试"}, {"config", "user.email", "test@example.com"}, {"commit", "--allow-empty", "-m", "基线"}} {
		command := exec.Command("git", args...)
		command.Dir = source
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	project, err := workspace.Register(ctx, "revision", workspace.Project{VCS: "git", Name: "检查点语义", Source: source, Branch: "main", WorkerID: "revision-worker"})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	request, err := submission.Prepare(ctx, "revision", "submit", run, submission.Snapshot{Workflow: workflow.Definition{ProjectID: project.ID, Input: "检查点语义", Nodes: []workflow.Node{}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = submission.Dispatch(ctx, "revision", request.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	if err = workspace.Ensure(ctx, run, project.ID, "revision"); err != nil {
		t.Fatal(err)
	}
	return run, project.ID
}

func TestReadOnlyCheckpointsExposeVerifiedRevision(t *testing.T) {
	run, project := revisionRun(t)
	ctx := context.Background()
	start, _ := workspace.Get(ctx, run)
	for i := 0; i < 2; i++ {
		input := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "plan", Executor: "codex", Instruction: "只读规划", ReadOnly: true}}
		item := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), Attempt: 1}
		_, err := executeNode(ctx, item, input, func(_ context.Context, in workflow.NodeInput) (workflow.NodeOutput, error) {
			if in.PlatformEvidence == nil {
				t.Fatal("未提供平台证据索引")
			}
			raw, _ := json.Marshal(in)
			var decoded struct {
				Revision *struct {
					OriginCommit string   `json:"origin_commit"`
					HeadCommit   string   `json:"head_commit"`
					ChangedFiles []string `json:"changed_files"`
				} `json:"revision"`
			}
			_ = json.Unmarshal(raw, &decoded)
			if decoded.Revision == nil || decoded.Revision.OriginCommit != start.BaseCommit || len(decoded.Revision.ChangedFiles) != 0 {
				t.Fatal("只读规划没有收到可信基线与文件差异")
			}
			if i == 1 && decoded.Revision.HeadCommit == start.BaseCommit {
				t.Fatal("未暴露当前检查点")
			}
			if _, err := os.Stat(filepath.Join(in.Workspace, ".git")); err != nil {
				t.Fatal(err)
			}
			return workflow.NodeOutput{Text: "现状核对完成"}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
