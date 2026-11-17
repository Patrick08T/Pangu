package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestEntryDefaultsOffAndChecksLiveActor(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testing.T, *entryFixture)
	}{
		{"policy-disabled", func(t *testing.T, f *entryFixture) { f.policy.Enabled = false; f.policy.Revision++; f.savePolicy(t) }},
		{"autopublish-disabled", func(t *testing.T, f *entryFixture) {
			f.policy.AutoPublish = false
			f.policy.Revision++
			f.savePolicy(t)
		}},
		{"policy-revoked", func(t *testing.T, f *entryFixture) { f.policy.Revoked = true; f.policy.Revision++; f.savePolicy(t) }},
		{"entry-not-authorized", func(t *testing.T, f *entryFixture) {
			f.policy.Entrypoints = []string{"workbench:other"}
			f.policy.Revision++
			f.savePolicy(t)
		}},
		{"viewer", func(t *testing.T, f *entryFixture) {
			entryExec(t, `UPDATE pangu.memberships SET role='viewer' WHERE user_id=$1`, f.actor.ID)
		}},
		{"disabled-user", func(t *testing.T, f *entryFixture) {
			entryExec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.actor.ID)
		}},
		{"foreign-namespace", func(t *testing.T, f *entryFixture) {
			f.actor.Namespace = "other"
			f.ctx = identity.WithActor(context.Background(), f.actor)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEntryFixture(t)
			tc.change(t, &f)
			_, err := f.submit("task", "denied")
			if !errors.Is(err, identity.ErrForbidden) {
				t.Fatalf("unauthorized entry: %v", err)
			}
		})
	}
}

func TestEntryPreflightFailureDoesNotAccept(t *testing.T) {
	f := newEntryFixture(t)
	failure := errors.New("external preflight failed")
	for _, tc := range []struct {
		check          *workflow.Preflight
		err, errorWant error
	}{{nil, failure, failure}, {&workflow.Preflight{}, nil, ErrNotReady}, {nil, nil, ErrNotReady}} {
		check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) { return tc.check, tc.err }
		_, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "not-ready", entryHooks{check, entryPending})
		if !errors.Is(err, tc.errorWant) {
			t.Fatal("wrong preflight failure", err)
		}
	}
	if n := entrySubmissionCount(t, f); n != 0 {
		t.Fatal("failed preflight persisted", n)
	}
}

func TestEntryBindingChangeDuringPreflightCannotAcceptStaleVersion(t *testing.T) {
	f := newEntryFixture(t)
	check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
		entryExec(t, `UPDATE pangu.evolution_bindings SET revision=revision+1 WHERE namespace=$1`, f.actor.Namespace)
		return &workflow.Preflight{Ready: true}, nil
	}
	_, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "cas", entryHooks{check, entryPending})
	if !errors.Is(err, submission.ErrConflict) {
		t.Fatal("binding CAS not enforced", err)
	}
	if n := entrySubmissionCount(t, f); n != 0 {
		t.Fatal("stale binding accepted", n)
	}
}

func TestEntryPermissionRevokedDuringPreflightDoesNotAccept(t *testing.T) {
	f := newEntryFixture(t)
	check := func(context.Context, workflow.Definition) (*workflow.Preflight, error) {
		entryExec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1`, f.actor.ID)
		return &workflow.Preflight{Ready: true}, nil
	}
	_, err := submitEntry(f.ctx, f.actor.Namespace, f.project, "main", "task", "revoked", entryHooks{check, entryPending})
	if !errors.Is(err, identity.ErrForbidden) {
		t.Fatal("permission revocation ignored", err)
	}
	if n := entrySubmissionCount(t, f); n != 0 {
		t.Fatal("revoked actor accepted", n)
	}
}

func entrySubmissionCount(t *testing.T, f entryFixture) int {
	t.Helper()
	var n int
	if err := entryQuery(t, `SELECT count(*) FROM pangu.submissions WHERE namespace=$1 AND operation='entry'`, f.actor.Namespace).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
