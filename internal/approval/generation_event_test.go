package approval

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestApprovalInvalidationEventRejectsStaleAcknowledgement(t *testing.T) {
	for _, status := range []string{"approved", "consumed", "changes_requested"} {
		t.Run(status, func(t *testing.T) {
			actor, _, run, input, initial := generationFixture(t, status)
			ctx := context.Background()
			before, err := Get(ctx, actor.Namespace, initial.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked"); err != nil {
				t.Fatal(err)
			}
			if err = SignalledState(ctx, actor.Namespace, before); !errors.Is(err, ErrConflict) {
				t.Fatal("stale receipt acknowledged new invalidation", err)
			}
			pending, err := PendingEvents(ctx, actor.Namespace)
			if err != nil || !slices.Contains(pending, initial.ID) {
				t.Fatal("invalidation wakeup lost", pending, err)
			}
			after, err := Get(ctx, actor.Namespace, initial.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = SignalledState(ctx, actor.Namespace, after); err != nil {
				t.Fatal(err)
			}
			pending, err = PendingEvents(ctx, actor.Namespace)
			if err != nil || slices.Contains(pending, initial.ID) {
				t.Fatal("current invalidation receipt not acknowledged", pending, err)
			}
		})
	}
}
