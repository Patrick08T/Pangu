package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"

	"github.com/Patrick08T/Pangu/internal/submission"
)

func entryReady(context.Context, workflow.Definition) (*workflow.Preflight, error) {
	return &workflow.Preflight{Ready: true}, nil
}
func entryPending(_ context.Context, r *submission.Record) *submission.Record { return r }
func (f entryFixture) submit(input, key string) (*submission.Record, error) {
	return submitEntry(f.ctx, f.actor.Namespace, f.project, "main", input, key, entryHooks{entryReady, entryPending})
}

func TestEntryPersistsBusinessProofAndFrozenAcceptance(t *testing.T) {
	f := newEntryFixture(t)
	got, err := f.submit("inspect this project", "request")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := submission.Get(f.ctx, f.actor.Namespace, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := stored.Snapshot
	if stored.Operation != "entry" || stored.CreatedBy != f.actor.ID || s.Purpose != "business" || s.EntryKey != "workbench:main" || s.EntryRevision != 1 || s.Workflow.VersionID != f.version.ID || s.Workflow.Input != "inspect this project" {
		t.Fatalf("incorrect durable entry identity: %+v", stored)
	}
	var count int
	if err = entryQuery(t, `SELECT count(*) FROM pangu.experience_watermarks WHERE namespace=$1 AND run_id=$2`, f.actor.Namespace, got.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("acceptance did not freeze atomically", count, err)
	}
}

func TestEntryRetryKeepsFrozenVersionAndRejectsChangedRequest(t *testing.T) {
	f := newEntryFixture(t)
	first, err := f.submit("task", "retry")
	if err != nil {
		t.Fatal(err)
	}
	entryExec(t, `UPDATE pangu.evolution_bindings SET revision=revision+1 WHERE namespace=$1`, f.actor.Namespace)
	f.policy.Revision++
	f.policy.Enabled = false
	f.savePolicy(t)
	got, err := f.submit("task", "retry")
	if err != nil || got.ID != first.ID || got.Snapshot.EntryRevision != 1 {
		t.Fatal("retry followed current binding or policy", got, err)
	}
	if _, err = f.submit("different", "retry"); !errors.Is(err, submission.ErrConflict) {
		t.Fatal("input change did not conflict", err)
	}
	_, err = submitEntry(f.ctx, f.actor.Namespace, f.project, "other", "task", "retry", entryHooks{entryReady, entryPending})
	if !errors.Is(err, submission.ErrConflict) {
		t.Fatal("entry change did not conflict", err)
	}
}

func TestEntrySubmitContractExists(t *testing.T) {
	f := newEntryFixture(t)
	var revision int64
	if err := entryQuery(t, `SELECT revision FROM pangu.evolution_bindings WHERE namespace=$1 AND project_id=$2 AND entry_key='workbench:main'`, f.actor.Namespace, f.project).Scan(&revision); err != nil || revision != 1 {
		t.Fatal("real binding fixture missing", err)
	}
	_, ok := any(Backend{}).(interface {
		SubmitEntry(context.Context, string, string, string, string) (*submission.Record, error)
	})
	if !ok {
		t.Fatal("Backend lacks authorized workbench SubmitEntry")
	}
}
