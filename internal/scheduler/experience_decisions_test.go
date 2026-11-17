package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func approvedEvidenceRuntimeFixture(t *testing.T) (*nodeExperience, workflow.NodeInput) {
	t.Helper()
	ctx := context.Background()
	r, input, out := evidenceRuntimeGenerationFixture(t, 1)
	_, err := submission.Dispatch(ctx, r.namespace, r.request.ID, func(context.Context, *submission.Record) (string, error) { return r.runID, nil })
	if err != nil {
		t.Fatal(err)
	}
	gateInput, err := r.EvidenceApprovalInput(ctx, input, out)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := approval.Request(ctx, r.namespace, r.request.ID, r.runID, gateInput)
	if err != nil {
		t.Fatal(err)
	}
	// Seed a persisted human decision; the worker still reads and verifies the real subject/artifact.
	rebuildExec(t, `UPDATE pangu.approvals SET status='approved',reason='new evidence',decided_at=now() WHERE id=$1`, gate.ID)
	input.EvidenceRetry = &workflow.EvidenceRetry{ApprovalID: gate.ID, Round: 1, Reason: "new evidence", Items: []workflow.ReviewAnswer{}}
	return r, input
}

func TestExperiencedWorkerVerifiesBoundEvidenceDecisionPayload(t *testing.T) {
	r, input := approvedEvidenceRuntimeFixture(t)
	ctx := context.Background()
	if err := r.Prepare(ctx, &input); err != nil {
		t.Fatal(err)
	}
	item := execution.Attempt{RunID: r.runID, NodeRunID: uuid.NewString(), NodeID: input.Node.ID, Attempt: 1}
	calls := 0
	execute := func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		calls++
		return workflow.NodeOutput{Text: "verified"}, nil
	}
	forged := input
	retry := *input.EvidenceRetry
	retry.Reason = "forged authority"
	forged.EvidenceRetry = &retry
	if err := r.Prepare(ctx, &forged); err != nil {
		t.Fatal(err)
	}
	if _, err := executeExperienced(ctx, r.namespace, item, forged, execute); err == nil || calls != 0 {
		t.Fatal("forged decision body reached executor", calls, err)
	}
	// This fixture has no Git checkout; verify the shared pre-dispatch boundary directly.
	if err := r.checkDispatch(ctx, input); err != nil {
		t.Fatal("valid persisted evidence decision rejected", err)
	}
}
