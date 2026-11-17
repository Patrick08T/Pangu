//go:build integration

package learning

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
)

func TestLearningUsesCurrentMembershipAndActor(t *testing.T) {
	f := newLearningFixture(t)
	if _, err := f.store.SavePolicy(f.ctx, f.scope, experience.Policy{}, 0); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE pangu.memberships SET role='viewer' WHERE user_id=$1`, f.actor.ID)
	if _, err := f.store.Policy(f.ctx, f.scope); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.SavePolicy(f.ctx, f.scope, experience.Policy{}, 1); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("viewer changed learning policy", err)
	}
	f.exec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1`, f.actor.ID)
	if _, err := f.store.Policy(f.ctx, f.scope); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("removed member read", err)
	}
}

func TestLearningRejectsImpersonationUnknownActionAndDisabledActor(t *testing.T) {
	f := newLearningFixture(t)
	scope := f.scope
	scope.ActorID = uuid.NewString()
	if _, err := f.store.Policy(f.ctx, scope); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("impersonation", err)
	}
	scope = f.scope
	scope.Action = "new_unreviewed_action"
	if err := f.store.Authorize(f.ctx, f.transaction(t), scope); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("unknown action", err)
	}
	f.exec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.actor.ID)
	if _, err := f.store.Policy(f.ctx, f.scope); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("disabled actor", err)
	}
}

func TestLearningRechecksTokenAndNeverAcceptsTokenPolicyWrites(t *testing.T) {
	f := newLearningFixture(t)
	ctx := f.token(t, []string{"read", "run", "manage_connections"})
	if _, err := f.store.Policy(ctx, f.scope); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.SavePolicy(ctx, f.scope, experience.Policy{}, 0); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("token changed opt-in policy", err)
	}
	f.exec(t, `UPDATE pangu.api_tokens SET revoked_at=now() WHERE id=$1`, identity.Current(ctx).TokenID)
	if _, err := f.store.Policy(ctx, f.scope); !errors.Is(err, experience.ErrForbidden) {
		t.Fatal("revoked token read", err)
	}
}
