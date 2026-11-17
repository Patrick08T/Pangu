package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

func TestRebuildRejectsRevokedTokenEvenWithPreviouslyAuthenticatedActor(t *testing.T) {
	f := newRebuildFixture(t)
	scopes := []string{"run", "manage_run"}
	token, _, err := identity.IssueToken(f.ctx, f.actor, f.previous.ProjectID, "rebuild test", scopes, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	actor := f.actor
	actor.TokenID = token.ID
	actor.ProjectID = f.previous.ProjectID
	actor.Scopes = scopes
	if err = identity.RevokeToken(f.ctx, f.actor, token.ID); err != nil {
		t.Fatal(err)
	}
	_, err = rebuildInvalidated(identity.WithActor(f.ctx, actor), f.actor.Namespace, f.run, f.intent.ID, f.stopped, func(context.Context, *submission.Record) (string, error) {
		t.Fatal("revoked token dispatched")
		return "", nil
	})
	if !errors.Is(err, identity.ErrForbidden) || rebuildCount(t, f.actor.Namespace) != 0 {
		t.Fatal("stale authenticated token retained authority", err)
	}
}

func TestRebuildChecksStopAgainAfterAtomicPreparation(t *testing.T) {
	f := newRebuildFixture(t)
	lookup := func(ctx context.Context, run string) (*RunDetails, error) {
		linked, err := approval.RebuildSuccessor(ctx, f.actor.Namespace, f.intent.ID)
		if err != nil {
			return nil, err
		}
		if linked != "" {
			return &RunDetails{RunSummary: RunSummary{ID: run, Status: "unknown"}}, nil
		}
		return f.stopped(ctx, run)
	}
	_, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, lookup, func(context.Context, *submission.Record) (string, error) {
		t.Fatal("dispatch bypassed fresh stop check")
		return "", nil
	})
	if err == nil || rebuildCount(t, f.actor.Namespace) != 1 {
		t.Fatal("dispatch stop barrier missing", err)
	}
	linked, err := approval.RebuildSuccessor(f.ctx, f.actor.Namespace, f.intent.ID)
	if err != nil || linked == "" {
		t.Fatal("prepared successor lost link", err)
	}
}

func TestRebuildRejectsMembershipRevokedAfterPreparation(t *testing.T) {
	f := newRebuildFixture(t)
	lookup := func(ctx context.Context, run string) (*RunDetails, error) {
		linked, err := approval.RebuildSuccessor(ctx, f.actor.Namespace, f.intent.ID)
		if err != nil {
			return nil, err
		}
		if linked != "" {
			rebuildExec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1`, f.actor.ID)
		}
		return f.stopped(ctx, run)
	}
	pending, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, f.intent.ID, lookup, func(context.Context, *submission.Record) (string, error) {
		t.Fatal("membership revoked between prepare and send retained authority")
		return "", nil
	})
	if !errors.Is(err, identity.ErrForbidden) || pending == nil || pending.RunID != "" || rebuildCount(t, f.actor.Namespace) != 1 {
		t.Fatal("dispatch authority was not revalidated", pending, err)
	}
}

func TestRebuildRejectsWrongIntentRunAssociation(t *testing.T) {
	f := newRebuildFixture(t)
	_, err := rebuildInvalidated(f.ctx, f.actor.Namespace, f.run, uuid.NewString(), f.stopped, func(context.Context, *submission.Record) (string, error) {
		t.Fatal("unknown intent dispatched")
		return "", nil
	})
	if !errors.Is(err, approval.ErrNotFound) || rebuildCount(t, f.actor.Namespace) != 0 {
		t.Fatal("intent association ignored", err)
	}
}
