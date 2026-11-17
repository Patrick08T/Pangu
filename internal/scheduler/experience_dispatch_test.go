package scheduler

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func TestDispatchFreezesChangedEvidenceInputBeforeExternalCall(t *testing.T) {
	runtime, input := approvedEvidenceRuntimeFixture(t)
	ns, request, run := runtime.namespace, runtime.request, runtime.runID
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	calls := 0
	dispatch := func() error {
		calls++
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2`, ns, request.ProjectID).Scan(&n); err != nil {
			return err
		}
		if n != 2 {
			t.Fatalf("external dispatch preceded changed-input freeze: usages=%d", n)
		}
		return nil
	}
	if err = dispatchExperienced(ctx, ns, run, &input, dispatch); err != nil {
		t.Fatal(err)
	}
	input.Node.Instruction = "not accepted"
	if err = dispatchExperienced(ctx, ns, run, &input, dispatch); err == nil || calls != 1 {
		t.Fatal("changed definition reached external dispatch", calls, err)
	}
}

func TestFinishedLayerExposesFinalEvidenceInputToRootObserver(t *testing.T) {
	inputs := []workflow.NodeInput{{Node: workflow.Node{ID: "source"}}}
	job := &nodeDispatch{input: inputs[0], index: 0}
	err := finishDispatchedLayer(inputs, []*nodeDispatch{job}, func() error {
		job.input.EvidenceRetry = &workflow.EvidenceRetry{ApprovalID: "approved-evidence", Round: 2}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if inputs[0].EvidenceRetry == nil || inputs[0].EvidenceRetry.ApprovalID != "approved-evidence" {
		t.Fatal("root still observes pre-evidence logical input")
	}
}
