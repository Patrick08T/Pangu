//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/google/uuid"
)

func TestEvolutionRecoveryRequiresActualLocalStopAndQueriesAllPages(t *testing.T) {
	f, p, run := evolutionStartedFixture(t, "http://127.0.0.1:1")
	ctx := context.Background()
	items, err := evolutionPendingPage(ctx, f.scope.Namespace, "")
	if err != nil || len(items) != 1 {
		t.Fatalf("pending intent page: %v %v", items, err)
	}
	rest, err := evolutionPendingPage(ctx, f.scope.Namespace, items[0].ID)
	if err != nil || len(rest) != 0 {
		t.Fatalf("pending cursor repeated intent: %v %v", rest, err)
	}
	runs, err := evolutionObservationPage(ctx, f.scope, p.Key.IntentID, p.Key.Generation, "")
	if err != nil || len(runs) != 0 {
		t.Fatalf("evaluation selected as business observation: %v %v", runs, err)
	}
	attempt := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: p.Definition.Nodes[0].ID}
	if err = execution.Begin(ctx, &attempt); err != nil {
		t.Fatal(err)
	}
	if stopped, e := evolutionAbandonCase(ctx, f.scope, p.RequestID, run); e != nil || stopped {
		t.Fatalf("active local attempt declared stopped: %v %v", stopped, e)
	}
	attempt.Status = "failed"
	if err = execution.Finish(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	if stopped, e := evolutionAbandonCase(ctx, f.scope, p.RequestID, run); e != nil || !stopped {
		t.Fatalf("stopped local attempt not reconciled: %v %v", stopped, e)
	}
}
