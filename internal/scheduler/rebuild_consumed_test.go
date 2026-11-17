package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func TestRebuildConsumedApprovalKeepsHistoryAndRejectsOldExecution(t *testing.T) {
	f := newRebuildFixture(t, true)
	if !f.intent.RequiresStop {
		t.Fatal("consumed approval lost required stop evidence")
	}
	prior, err := approval.Get(f.ctx, f.actor.Namespace, f.gate.ID)
	if err != nil || prior.Status != "consumed" || prior.InvalidatedAt == nil {
		t.Fatal("consumed history was rewritten", prior, err)
	}
	child := uuid.NewString()
	next, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) { return child, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = execution.Begin(f.ctx, &execution.Attempt{RunID: f.run, NodeRunID: uuid.NewString(), NodeID: "build"}); !errors.Is(err, execution.ErrCancelled) {
		t.Fatal("old run resumed after rebuild", err)
	}
	gate, err := approval.Request(f.ctx, f.actor.Namespace, next.ID, child, f.input)
	if err != nil || gate.Status != "waiting" || gate.InheritedFrom != "" {
		t.Fatal("consumed approval bypassed fresh planning approval", gate, err)
	}
}
