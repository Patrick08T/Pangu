package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestEntryAcceptanceRaceReusesWinnerBeforeRecheckingBinding(t *testing.T) {
	f := newEntryFixture(t)
	var winner *submission.Record
	check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
		var err error
		winner, err = f.submit("task", "winner")
		if err != nil {
			return nil, err
		}
		entryExec(t, `UPDATE pangu.evolution_bindings SET revision=revision+1 WHERE namespace=$1`, f.actor.Namespace)
		f.policy.Revision++
		f.policy.Enabled = false
		f.savePolicy(t)
		return &workflow.Preflight{Ready: true}, nil
	}
	got, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "winner", entryHooks{check, entryPending})
	if err != nil || got.ID != winner.ID || got.Snapshot.EntryRevision != 1 {
		t.Fatal("late retry followed changed authority rather than accepted winner", got, err)
	}
	if n := entrySubmissionCount(t, f); n != 1 {
		t.Fatal("winner duplicated", n)
	}
}

func TestEntryAcceptanceRaceWithDifferentInputConflicts(t *testing.T) {
	f := newEntryFixture(t)
	check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
		_, err := f.submit("different input", "winner")
		return &workflow.Preflight{Ready: true}, err
	}
	_, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "winner", entryHooks{check, entryPending})
	if !errors.Is(err, submission.ErrConflict) {
		t.Fatal("late conflicting acceptance was returned as retry", err)
	}
	if n := entrySubmissionCount(t, f); n != 1 {
		t.Fatal("conflicting request persisted", n)
	}
}
