//go:build integration

package approval

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type continuationMemoryFixture struct {
	ctx        context.Context
	actor      identity.Actor
	scope      experience.Scope
	role       string
	definition workflow.Definition
	version    experience.Version
	source     experience.Source
	checker    string
	intent     experience.LearningIntent
}

func newContinuationMemoryFixture(t *testing.T, empty ...bool) continuationMemoryFixture {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("requires isolated integration database")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	actor := identity.Actor{ID: uuid.NewString(), Namespace: uuid.NewString()}
	f := continuationMemoryFixture{ctx: identity.WithActor(context.Background(), actor), actor: actor,
		scope: experience.Scope{Namespace: actor.Namespace, ActorID: actor.ID, ProjectID: uuid.NewString()}}
	f.exec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, actor.ID, actor.Namespace)
	f.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'inheritance','unused','main','test','git')`, f.scope.ProjectID, actor.Namespace)
	f.define(t)
	if len(empty) == 0 || !empty[0] {
		f.activate(t)
	}
	return f
}

func (f continuationMemoryFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(f.ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f *continuationMemoryFixture) define(t *testing.T) {
	t.Helper()
	h := &workflow.HandoffContract{Objective: "目标", Inputs: "输入", Deliverables: "报告", Acceptance: "验证", FailureHandling: "暂停"}
	agent, err := catalog.Create(f.ctx, f.actor.Namespace, catalog.NewObject{Kind: "agent", Name: "执行角色", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Instruction: "执行", Handoff: h}}})
	if err != nil {
		t.Fatal(err)
	}
	f.role = agent.ID
	av, err := catalog.Publish(f.ctx, f.actor.Namespace, agent.ID, agent.Revision)
	if err != nil {
		t.Fatal(err)
	}
	flow := &catalog.Flow{Budget: &workflow.Budget{CodexSessions: 1}, Nodes: []catalog.Binding{
		{ID: "plan", AgentVersion: av.ID, ReadOnly: true},
		{ID: "build", AgentVersion: av.ID, DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}, ExecutionPlan: true}},
	}}
	object, err := catalog.Create(f.ctx, f.actor.Namespace, catalog.NewObject{Kind: "workflow", Name: "经验审批续跑", Spec: catalog.Spec{Flow: flow}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(f.ctx, f.actor.Namespace, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := catalog.Compile(f.ctx, f.actor.Namespace, version.ID, f.scope.ProjectID, "JSON方案")
	if err != nil {
		t.Fatal(err)
	}
	f.definition = *definition
}

func (f continuationMemoryFixture) start(t *testing.T) (*submission.Record, workflow.NodeInput, *workflow.Fork) {
	t.Helper()
	item, err := submission.Prepare(f.ctx, f.actor.Namespace, "submit", uuid.NewString(), submission.Snapshot{Workflow: f.definition})
	if err != nil {
		t.Fatal(err)
	}
	item = f.dispatch(t, item)
	body := withReview(`{"tasks":[{"id":"T1"}],"execution_plan":{"changes":[{"node_id":"build","task":"实施具体获批任务"}],"assignments":[{"task_id":"T1","node_id":"build"}]}}`)
	output := workflow.NodeOutput{Text: body, RunID: uuid.NewString()}
	raw, _ := json.Marshal(output)
	f.exec(t, `INSERT INTO pangu.workspace_checkpoints(node_run_id,run_id,commit_id,output) VALUES($1,$2,'head',$3)`, output.RunID, item.RunID, raw)
	fork := &workflow.Fork{OriginCommit: "base", Commit: "head", CheckpointNode: "plan", Checkpoints: map[string]string{"plan": "head"}, Reused: map[string]workflow.NodeOutput{"plan": output}}
	input := item.Snapshot.Workflow.NodeInput(item.Snapshot.Workflow.Nodes[1], fork.Reused)
	return item, input, fork
}

func (f continuationMemoryFixture) dispatch(t *testing.T, item *submission.Record) *submission.Record {
	t.Helper()
	run := uuid.NewString()
	_, err := submission.Dispatch(f.ctx, f.actor.Namespace, item.ID, func(context.Context, *submission.Record) (string, error) { return run, nil })
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','test','unused',true)`, run, f.scope.ProjectID)
	item, err = submission.Get(f.ctx, f.actor.Namespace, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func (f continuationMemoryFixture) prepare(t *testing.T, item *submission.Record, input workflow.NodeInput) workflow.NodeInput {
	t.Helper()
	input.RequestID = item.ID
	if input.ApprovalRound == 0 {
		input.ApprovalRound = 1
	}
	if input.ApprovalGeneration == 0 {
		input.ApprovalGeneration = 1
	}
	input, err := item.Snapshot.Workflow.BindInput(input)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := submission.ExperienceSelection(item.ID, input, f.role, item.Operation)
	if err != nil {
		t.Fatal(err)
	}
	usage, versions, err := learning.NewStore().Select(f.ctx, f.scope, selection)
	if err != nil || (f.version.ID != "" && len(versions) == 0) {
		t.Fatalf("real selection failed: %+v %v", usage, err)
	}
	input.Experiences, err = submission.ExperiencePayload(usage, versions)
	if err != nil {
		t.Fatal(err)
	}
	return input
}
