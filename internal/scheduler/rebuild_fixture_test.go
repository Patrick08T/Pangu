package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

type rebuildFixture struct {
	ctx      context.Context
	actor    identity.Actor
	previous *submission.Record
	intent   *approval.RebuildIntent
	input    workflow.NodeInput
	gate     *approval.Gate
	run      string
}

func newRebuildFixture(t *testing.T, consumed ...bool) rebuildFixture {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要隔离测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", rebuildTemp(t))
	t.Setenv("PANGU_WORKER_ID", "rebuild-test")
	actor := identity.Actor{ID: uuid.NewString(), Namespace: "rebuild-" + uuid.NewString()}
	ctx := identity.WithActor(context.Background(), actor)
	project := rebuildProject(t, ctx, actor)
	review := workflow.Review{Title: "范围", Summary: "确认", Items: []workflow.ReviewItem{{ID: "scope", Category: "范围", Title: "文档", Proposal: "只修改文档", Rationale: "需求", TaskIDs: []string{}, EvidenceRefs: []string{}}}, Resolutions: []workflow.ReviewResolution{}}
	body, _ := json.Marshal(map[string]any{"review": review})
	node := workflow.Node{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}
	d := workflow.Definition{ProjectID: project.ID, Input: "task", Nodes: []workflow.Node{{ID: "plan", Executor: "codex", Instruction: "规划", ReadOnly: true}, node}}
	previous, err := submission.Prepare(ctx, actor.Namespace, "submit", "original", submission.Snapshot{Purpose: "business", Workflow: d})
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
	input := d.NodeInput(node, map[string]workflow.NodeOutput{"plan": {Text: string(body)}})
	gate, err := approval.Request(ctx, actor.Namespace, previous.ID, run, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = approval.Decide(ctx, actor, gate.ID, approval.Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}}}); err != nil {
		t.Fatal(err)
	}
	if len(consumed) > 0 && consumed[0] {
		if err = approval.Consume(ctx, run, input, gate.ID); err != nil {
			t.Fatal(err)
		}
	}
	intent, err := approval.Invalidate(ctx, actor.Namespace, run, node.ID, 1, "dependency revoked")
	if err != nil {
		t.Fatal(err)
	}
	return rebuildFixture{ctx, actor, previous, intent, input, gate, run}
}

func rebuildProject(t *testing.T, ctx context.Context, actor identity.Actor) *workspace.Project {
	t.Helper()
	source := rebuildTemp(t)
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "test"}, {"config", "user.email", "test@example.com"}, {"commit", "--allow-empty", "-m", "baseline"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v %s", err, output)
		}
	}
	project, err := workspace.Register(ctx, actor.Namespace, workspace.Project{VCS: "git", Name: "rebuild", Source: source, Branch: "main", WorkerID: "rebuild-test"})
	if err != nil {
		t.Fatal(err)
	}
	rebuildExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, actor.ID, actor.Namespace)
	rebuildExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role,enabled) VALUES($1,$2,'operator',true)`, actor.ID, project.ID)
	return project
}

func rebuildExec(t *testing.T, query string, args ...any) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f rebuildFixture) stopped(context.Context, string) (*RunDetails, error) {
	// Only the external Hatchet status lookup is substituted; local attempts/cancellation stay real.
	return &RunDetails{RunSummary: RunSummary{ID: f.run, ProjectID: f.previous.ProjectID, CreatedBy: f.actor.ID, Status: "cancelled"}, StopConfirmed: true, Nodes: []NodeRun{}}, nil
}

func rebuildTemp(t *testing.T) string {
	t.Helper()
	path, err := os.MkdirTemp("", "rb-")
	if err != nil {
		t.Fatal(err)
	}
	// The fresh OS-created directory is owned exclusively by this fixture.
	t.Cleanup(func() {
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	})
	return path
}

func rebuildCount(t *testing.T, namespace string) int {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var count int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.submissions WHERE namespace=$1 AND operation='rebuild'`, namespace).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
