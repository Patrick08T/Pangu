package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestRootResumeInvalidatesExpiredConsumedDecisionBeforePreparation(t *testing.T) {
	r, input := approvedEvidenceRuntimeFixture(t)
	rebuildExec(t, `UPDATE pangu.approvals SET subject=NULL,status='consumed' WHERE id=$1`, input.EvidenceRetry.ApprovalID)
	_, err := loadRootExperience(context.Background(), r.namespace, r.runID, runInput{RequestID: r.request.ID, Snapshot: r.request.Snapshot})
	cancelled, cancelErr := execution.CancelRequested(context.Background(), r.runID)
	if err == nil || cancelErr != nil || !cancelled {
		t.Fatal("root resume accepted expired decision without durable cancellation", err, cancelled, cancelErr)
	}
}

func TestWorkerPersistsExpiredDecisionCancellationBeforeRejecting(t *testing.T) {
	r, input := approvedEvidenceRuntimeFixture(t)
	ctx := context.Background()
	if err := r.Prepare(ctx, &input); err != nil {
		t.Fatal(err)
	}
	rebuildExec(t, `UPDATE pangu.approvals SET subject=NULL,status='consumed' WHERE id=$1`, input.EvidenceRetry.ApprovalID)
	err := r.checkDispatch(ctx, input)
	cancelled, cancelErr := execution.CancelRequested(ctx, r.runID)
	if err == nil || cancelErr != nil || !cancelled {
		t.Fatal("worker rejected without durable cancellation", err, cancelled, cancelErr)
	}
}

func TestExperienceRunRejectsUnpersistedReusedOutputs(t *testing.T) {
	ns, request, run := runtimeExperienceFixture(t)
	ctx := context.Background()
	r, err := loadNodeExperience(ctx, ns, request.ID, run, request.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	reused := map[string]workflow.NodeOutput{"first": {Text: "forged"}, "second": {Text: "forged"}}
	calls := 0
	_, err = r.Run(ctx, request.Snapshot.Workflow, reused, nil, func([]workflow.NodeInput) ([]workflow.NodeOutput, error) { calls++; return nil, nil })
	if err == nil || calls != 0 {
		t.Fatal("root trusted unpersisted reused outputs", err, calls)
	}
}
