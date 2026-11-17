package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func runtimeExperienceFixture(t *testing.T) (string, *submission.Record, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	namespace, project := "runtime-memory-"+uuid.NewString(), uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,$2,'runtime memory','unused','main','test')`, project, namespace)
	if err != nil {
		t.Fatal(err)
	}
	definition := workflow.Definition{ProjectID: project, Input: "task", Nodes: []workflow.Node{{ID: "first", Executor: "eino", Model: "fixture", Instruction: "task"}, {ID: "second", Executor: "eino", Model: "fixture", Instruction: "task", DependsOn: []string{"first"}}}}
	request, err := submission.Prepare(ctx, namespace, "submit", "runtime", submission.Snapshot{Workflow: definition})
	if err != nil {
		t.Fatal(err)
	}
	return namespace, request, uuid.NewString()
}

func TestRuntimeExperienceCreatesValidatedEmptyDependencyChain(t *testing.T) {
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	runtime, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	input := request.Snapshot.Workflow.NodeInput(request.Snapshot.Workflow.Nodes[0], nil)
	if err = runtime.Prepare(ctx, &input); err != nil {
		t.Fatal(err)
	}
	if input.RequestID != request.ID || len(input.Experiences) != 0 {
		t.Fatalf("lost frozen request: %+v", input)
	}
	input.RunID, input.NodeRunID = run, uuid.NewString()
	actual := input
	actual.Revision = &workflow.Revision{OriginCommit: "origin", HeadCommit: "head"}
	out := workflow.NodeOutput{RunID: input.NodeRunID, Text: "result"}
	if err = runtime.Record(ctx, input, actual, &out); err != nil {
		t.Fatal(err)
	}
	if out.Lineage == nil {
		t.Fatal("successful result has no proof")
	}
	var envelope map[string]any
	raw, _ := json.Marshal(out.Lineage)
	_ = json.Unmarshal(raw, &envelope)
	if envelope["execution_input_hash"] == nil || envelope["execution_input_hash"] == envelope["input_hash"] {
		t.Fatalf("resolved execution context not independently recorded: %s", raw)
	}
	child := request.Snapshot.Workflow.NodeInput(request.Snapshot.Workflow.Nodes[1], map[string]workflow.NodeOutput{"first": out})
	if err = runtime.Prepare(ctx, &child); err != nil {
		t.Fatal(err)
	}
	out.Text = "tampered"
	child.Parents["first"] = out
	if err = runtime.Prepare(ctx, &child); err == nil {
		t.Fatal("changed upstream content bypassed lineage validation")
	}
}

func TestRuntimeExperienceRejectsUnboundIdentityAndLegacyProof(t *testing.T) {
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	if _, err := loadNodeExperience(ctx, ns, request.ID, run, uuid.NewString()); err == nil {
		t.Fatal("cross-project request accepted")
	}
	runtime, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	input := request.Snapshot.Workflow.NodeInput(request.Snapshot.Workflow.Nodes[1], map[string]workflow.NodeOutput{"first": {Skipped: true, RunID: uuid.NewString(), Text: "legacy"}})
	if err = runtime.Prepare(ctx, &input); err == nil {
		t.Fatal("legacy skipped parent accepted without proof")
	}
	input = request.Snapshot.Workflow.NodeInput(request.Snapshot.Workflow.Nodes[0], nil)
	actorCtx := identity.WithActor(ctx, identity.Actor{ID: uuid.NewString(), Namespace: ns})
	if err = runtime.Prepare(actorCtx, &input); err == nil {
		t.Fatal("named actor obtained anonymous runtime capability")
	}
	input.Node.Instruction = "different accepted node"
	if err = runtime.Prepare(ctx, &input); err == nil {
		t.Fatal("mutated frozen node accepted")
	}
}
