package scheduler

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func TestRebuildCreatesOneFreshSuccessorWithoutClearingCancellation(t *testing.T) {
	f := newRebuildFixture(t)
	child := uuid.NewString()
	send := func(ctx context.Context, item *submission.Record) (string, error) {
		linked, err := approval.RebuildSuccessor(ctx, f.actor.Namespace, f.intent.ID)
		if err != nil || linked != item.ID {
			t.Fatalf("dispatch observed an orphan successor: %s %v", linked, err)
		}
		return child, nil
	}
	var first *submission.Record
	for range 2 {
		next, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, send)
		if err != nil {
			t.Fatalf("explicit stopped rebuild rejected: %v", err)
		}
		if first != nil && first.ID != next.ID {
			t.Fatal("replay created another successor")
		}
		first = next
		if next.RunID != child || next.CreatedBy != f.actor.ID || next.Snapshot.SourceRunID != f.run || next.Snapshot.Fork != nil || next.Snapshot.ApprovalOrigin != "" {
			t.Fatalf("unclean successor: %+v", next)
		}
		if !reflect.DeepEqual(next.Snapshot.Workflow, f.previous.Snapshot.Workflow) {
			t.Fatal("rebuild changed frozen workflow")
		}
	}
	cancelled, err := execution.CancelRequested(f.ctx, f.run)
	if err != nil || !cancelled {
		t.Fatal("original cancellation was cleared", err)
	}
	if count := rebuildCount(t, f.actor.Namespace); count != 1 {
		t.Fatal("orphan/duplicate successor", count)
	}
	gate, err := approval.Request(f.ctx, f.actor.Namespace, first.ID, child, f.input)
	if err != nil || gate.Status != "waiting" || gate.InheritedFrom != "" || gate.ApprovalGeneration != 1 {
		t.Fatalf("new run reused old approval: %+v %v", gate, err)
	}
	if err = approval.Consume(f.ctx, f.run, f.input, f.gate.ID); !errors.Is(err, approval.ErrConflict) {
		t.Fatal("old decision revived", err)
	}
}

func TestRebuildRejectsUnknownStopAndActiveLocalAttempt(t *testing.T) {
	for _, scenario := range []string{"unknown", "active", "lookup failure"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRebuildFixture(t)
			lookup := f.stopped
			if scenario == "unknown" {
				lookup = func(context.Context, string) (*RunDetails, error) {
					return &RunDetails{RunSummary: RunSummary{ID: f.run, Status: "unknown"}, StopConfirmed: true}, nil
				}
			}
			if scenario == "lookup failure" {
				lookup = func(context.Context, string) (*RunDetails, error) { return nil, errors.New("remote unavailable") }
			}
			if scenario == "active" {
				rebuildExec(t, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt,owner_host,owner_pid,owner_stamp,process_scope) VALUES($1,$2,'build',1,'not-this-host',1,'unknown','')`, f.run, uuid.NewString())
			}
			_, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, lookup, func(context.Context, *submission.Record) (string, error) {
				t.Fatal("unsafe external dispatch")
				return "", nil
			})
			if err == nil || rebuildCount(t, f.actor.Namespace) != 0 {
				t.Fatal("unknown/active execution created successor", err)
			}
		})
	}
}

func TestRebuildRejectsRevokedOwnerAndManagerImpersonation(t *testing.T) {
	for _, scenario := range []string{"revoked", "manager", "anonymous"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRebuildFixture(t)
			ctx := f.ctx
			if scenario == "revoked" {
				rebuildExec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1`, f.actor.ID)
			}
			if scenario == "manager" {
				other := identity.Actor{ID: uuid.NewString(), Namespace: f.actor.Namespace}
				rebuildExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, other.ID, other.Namespace)
				ctx = identity.WithActor(ctx, other)
			}
			if scenario == "anonymous" {
				ctx = context.Background()
			}
			_, err := rebuildInvalidated(ctx, f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) {
				t.Fatal("unauthorized dispatch")
				return "", nil
			})
			if !errors.Is(err, identity.ErrForbidden) || rebuildCount(t, f.actor.Namespace) != 0 {
				t.Fatal("unauthorized rebuild accepted", err)
			}
		})
	}
}
