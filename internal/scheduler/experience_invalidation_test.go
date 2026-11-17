package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type experienceInvalidationFixture struct {
	ns, run, project, actor, version, source string
	ctx                                      context.Context
	legacy                                   bool
	request                                  *submission.Record
	runtime                                  *nodeExperience
	input                                    workflow.NodeInput
	parent                                   workflow.NodeOutput
	gate                                     *approval.Gate
}

func newExperienceInvalidationFixture(t *testing.T, setup ...func(*experienceInvalidationFixture, *workflow.Definition)) experienceInvalidationFixture {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	f := experienceInvalidationFixture{ns: "approval-memory-" + uuid.NewString(), run: uuid.NewString(), project: uuid.NewString(), ctx: context.Background()}
	rebuildExec(t, `INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,$2,'approval memory','unused','main','test')`, f.project, f.ns)
	d := workflow.Definition{ProjectID: f.project, Input: "task", Nodes: []workflow.Node{
		{ID: "plan", Executor: "codex", Instruction: "规划", ReadOnly: true},
		{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}},
	}}
	for _, fn := range setup {
		fn(&f, &d)
	}
	if f.actor != "" {
		f.ctx = identity.WithActor(f.ctx, identity.Actor{ID: f.actor, Namespace: f.ns})
	}
	var err error
	f.request, err = submission.Prepare(f.ctx, f.ns, "submit", "original", submission.Snapshot{Workflow: d, CreatedBy: f.actor})
	if err != nil {
		t.Fatal(err)
	}
	f.request, err = submission.Dispatch(f.ctx, f.ns, f.request.ID, func(context.Context, *submission.Record) (string, error) { return f.run, nil })
	if err != nil {
		t.Fatal(err)
	}
	f.runtime, err = loadNodeExperience(f.ctx, f.ns, f.request.ID, f.run, f.project)
	if err != nil {
		t.Fatal(err)
	}
	prepareExperienceInvalidationGate(t, &f, d)
	return f
}

func prepareExperienceInvalidationGate(t *testing.T, f *experienceInvalidationFixture, d workflow.Definition) {
	t.Helper()
	in := d.NodeInput(d.Nodes[0], nil)
	if err := f.runtime.Prepare(f.ctx, &in); err != nil {
		t.Fatal(err)
	}
	in.RunID, in.NodeRunID = f.run, uuid.NewString()
	body, _ := json.Marshal(map[string]any{"review": workflow.Review{Title: "范围", Summary: "确认", Items: []workflow.ReviewItem{{ID: "scope", Category: "范围", Title: "文档", Proposal: "只改文档", Rationale: "需求", TaskIDs: []string{}, EvidenceRefs: []string{}}}, Resolutions: []workflow.ReviewResolution{}}})
	f.parent = workflow.NodeOutput{Text: string(body)}
	if err := f.runtime.Record(f.ctx, in, in, &f.parent); err != nil {
		t.Fatal(err)
	}
	f.input = d.NodeInput(d.Nodes[1], map[string]workflow.NodeOutput{"plan": f.parent})
	f.input.ApprovalRound = 1
	if !f.legacy {
		if err := f.runtime.Prepare(f.ctx, &f.input); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	f.gate, err = approval.Request(f.ctx, f.ns, f.request.ID, f.run, f.input)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExperienceInvalidationRevokedParentCancelsWaitingGate(t *testing.T) {
	f := newExperienceInvalidationMemoryFixture(t)
	rebuildExec(t, `UPDATE pangu.experience_states SET validity='revoked' WHERE version_id=$1`, f.version)
	err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID)
	if !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("revoked approval remained usable: %v", err)
	}
	assertExperienceInvalidation(t, f, "invalidated")
}

func assertExperienceInvalidation(t *testing.T, f experienceInvalidationFixture, status string) {
	t.Helper()
	gate, err := approval.Get(context.Background(), f.ns, f.gate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gate.Status != status || gate.InvalidatedAt == nil {
		t.Fatalf("missing system invalidation: %+v", gate)
	}
	cancelled, err := execution.CancelRequested(context.Background(), f.run)
	if err != nil || !cancelled {
		t.Fatalf("not durably cancelled: %v %v", cancelled, err)
	}
	intents, err := approval.RebuildIntents(context.Background(), f.ns, f.run)
	if err != nil || len(intents) != 1 || intents[0].FromGeneration != 1 {
		t.Fatalf("missing unique generation intent: %+v %v", intents, err)
	}
}

func TestExperienceInvalidationHealthyAndContextFailureRemainWaiting(t *testing.T) {
	f := newExperienceInvalidationFixture(t)
	if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateExperienceApproval(ctx, f.ns, f.gate.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	gate, err := approval.Get(context.Background(), f.ns, f.gate.ID)
	if err != nil || gate.Status != "waiting" || gate.InvalidatedAt != nil {
		t.Fatalf("operational failure invalidated gate: %+v %v", gate, err)
	}
}

func TestExperienceInvalidationLegacySelectionFailsClosed(t *testing.T) {
	f := newExperienceInvalidationFixture(t, func(f *experienceInvalidationFixture, _ *workflow.Definition) { f.legacy = true })
	if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("legacy selection acquired proof: %v", err)
	}
	assertExperienceInvalidation(t, f, "invalidated")
}

func TestExperienceInvalidationSQLFailureDoesNotInvalidate(t *testing.T) {
	f := newExperienceInvalidationFixture(t)
	t.Setenv("PANGU_DATABASE_URL", "postgres://pangu:pangu@127.0.0.1:1/pangu?sslmode=disable&connect_timeout=1")
	err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID)
	if err == nil || errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("SQL failure became invalidation: %v", err)
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var count int
	if err = conn.QueryRow(context.Background(), `SELECT count(*) FROM pangu.approval_rebuild_intents WHERE namespace=$1`, f.ns).Scan(&count); err != nil || count != 0 {
		t.Fatalf("SQL failure created rebuild: %d %v", count, err)
	}
}
