package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func approveExperienceFixture(t *testing.T, f experienceInvalidationFixture, consume bool) {
	t.Helper()
	actor := identity.Actor{ID: uuid.NewString(), Namespace: f.ns}
	rebuildExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, actor.ID, f.ns)
	if _, err := approval.Decide(context.Background(), actor, f.gate.ID, approval.Decision{SubjectHash: f.gate.SubjectHash, Decision: "approved", Items: []workflow.ReviewAnswer{{ItemID: "scope", Decision: "accept"}}}); err != nil {
		t.Fatal(err)
	}
	if consume {
		if err := approval.Consume(context.Background(), f.run, f.input, f.gate.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExperienceInvalidationApprovedButUnconsumedGate(t *testing.T) {
	f := newExperienceInvalidationMemoryFixture(t)
	approveExperienceFixture(t, f, false)
	rebuildExec(t, `UPDATE pangu.experience_states SET validity='revoked' WHERE version_id=$1`, f.version)
	if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("approved gate escaped audit: %v", err)
	}
	assertExperienceInvalidation(t, f, "invalidated")
	if err := approval.Consume(context.Background(), f.run, f.input, f.gate.ID); err == nil {
		t.Fatal("invalidated approved gate consumed")
	}
}

func TestExperienceInvalidationPreservesConsumedHistoryAndProducts(t *testing.T) {
	f := newExperienceInvalidationMemoryFixture(t)
	approveExperienceFixture(t, f, true)
	if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); err != nil {
		t.Fatalf("in-flight consumed gate without a published output rejected: %v", err)
	}
	input := f.input
	input.RunID, input.NodeRunID = f.run, uuid.NewString()
	out := workflow.NodeOutput{Text: "completed implementation", RunID: input.NodeRunID}
	artifact, err := outputArtifact(input, out)
	if err != nil {
		t.Fatal(err)
	}
	artifact.InvalidReason = "legacy missing proof"
	raw, _ := json.Marshal(artifact)
	rebuildExec(t, `INSERT INTO pangu.experience_artifacts(namespace,project_id,id,artifact) VALUES($1,$2,$3,$4)`, f.ns, f.project, input.NodeRunID, raw)
	if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("consumed product escaped audit: %v", err)
	}
	assertExperienceInvalidation(t, f, "consumed")
	gate, err := approval.Get(context.Background(), f.ns, f.gate.ID)
	if err != nil || gate.ConsumedAt == nil || gate.DecidedAt == nil || gate.ActorID == "" {
		t.Fatalf("history erased: %+v %v", gate, err)
	}
	if err = approval.Consume(context.Background(), f.run, f.input, f.gate.ID); err == nil {
		t.Fatal("invalidated consumed gate replayed")
	}
}

func TestExperienceInvalidationExpiredConsumedSubjectStillStops(t *testing.T) {
	f := newExperienceInvalidationMemoryFixture(t)
	approveExperienceFixture(t, f, true)
	rebuildExec(t, `UPDATE pangu.approvals SET subject=NULL WHERE id=$1`, f.gate.ID)
	if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); !errors.Is(err, approval.ErrExpired) {
		t.Fatalf("lost retention failure: %v", err)
	}
	stopped, err := execution.CancelRequested(context.Background(), f.run)
	if err != nil || !stopped {
		t.Fatalf("expired consumed subject left run uncancelled: %v %v", stopped, err)
	}
	intents, err := approval.RebuildIntents(context.Background(), f.ns, f.run)
	if err != nil || len(intents) != 0 {
		t.Fatalf("expired subject fabricated rebuild authority: %+v %v", intents, err)
	}
	gate, err := approval.Get(context.Background(), f.ns, f.gate.ID)
	if err != nil || gate.Status != "consumed" || gate.ConsumedAt == nil {
		t.Fatalf("consumed history rewritten: %+v %v", gate, err)
	}
}

func TestExperienceInvalidationRepairsMissingCancellationIdempotently(t *testing.T) {
	f := newExperienceInvalidationFixture(t)
	if _, err := approval.Invalidate(context.Background(), f.ns, f.run, f.gate.NodeID, 1, "prior invalidation"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := validateExperienceApproval(context.Background(), f.ns, f.gate.ID); !errors.Is(err, experience.ErrInvalidated) {
			t.Fatalf("invalidation repair failed: %v", err)
		}
	}
	assertExperienceInvalidation(t, f, "invalidated")
}

func TestExperienceInvalidationSyntheticReviewAuditsOriginalArtifact(t *testing.T) {
	f := newExperienceInvalidationMemoryFixture(t)
	in := workflow.NodeInput{ReviewPurpose: "evidence", ProjectID: f.request.ProjectID, Input: f.input.Input, Node: workflow.Node{ID: "evidence-" + f.parent.RunID, Executor: "codex", Instruction: "补证后只读复验 plan", ReadOnly: true, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}, Parents: map[string]workflow.NodeOutput{"plan": {Text: f.parent.Text}}}
	gate, err := approval.Request(context.Background(), f.ns, f.request.ID, f.run, in)
	if err != nil {
		t.Fatal(err)
	}
	if err = validateExperienceApproval(context.Background(), f.ns, gate.ID); err != nil {
		t.Fatalf("valid synthetic review rejected: %v", err)
	}
	rebuildExec(t, `UPDATE pangu.experience_states SET validity='revoked' WHERE version_id=$1`, f.version)
	if err = validateExperienceApproval(context.Background(), f.ns, gate.ID); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatalf("synthetic review ignored original proof: %v", err)
	}
	f.gate = gate
	assertExperienceInvalidation(t, f, "invalidated")
}

func TestExperienceInvalidationNamedCallerCannotBorrowAnonymousCapability(t *testing.T) {
	f := newExperienceInvalidationFixture(t)
	ctx := identity.WithActor(context.Background(), identity.Actor{ID: uuid.NewString(), Namespace: f.ns})
	if err := validateExperienceApproval(ctx, f.ns, f.gate.ID); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("unbound caller accepted: %v", err)
	}
	cancelled, err := execution.CancelRequested(context.Background(), f.run)
	if err != nil || cancelled {
		t.Fatalf("caller mismatch damaged accepted gate: %v %v", cancelled, err)
	}
}
