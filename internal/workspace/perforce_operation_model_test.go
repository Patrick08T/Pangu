package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestPerforceOperationDoesNotHoldGitGuardDuringModel(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	called := false
	_, err := Execute(ctx, f.run.ID, uuid.NewString(), 1, true, func(string) (workflow.NodeOutput, error) {
		called = true
		if err := checkLocalGitOperation(context.Background()); err != nil {
			t.Fatalf("model callback inherited a completed Git phase's guard: %v", err)
		}
		return workflow.NodeOutput{}, workflow.ErrStopUnconfirmed
	})
	if !called || !errors.Is(err, workflow.ErrStopUnconfirmed) {
		t.Fatalf("model-stop result lost: called=%v error=%v", called, err)
	}
	if err := checkLocalGitOperation(context.Background()); err != nil {
		t.Fatalf("no running Git but model-stop uncertainty quarantined Git: %v", err)
	}
}
