//go:build integration

package learning

import (
	"github.com/Patrick08T/Pangu/internal/experience"
	"testing"
)

func TestLearningListsOnlyProjectAuthorizedCheckers(t *testing.T) {
	f := newLearningFixture(t)
	_, _, checker := f.claim(t)
	items, err := ListCheckers(f.ctx, f.scope, 10, 0)
	if err != nil || len(items) != 1 || items[0].ID != checker.ID {
		t.Fatal(items, err)
	}
	other := f.scope
	other.ActorID = newLearningFixture(t).actor.ID
	if _, err = ListCheckers(f.ctx, other, 10, 0); err == nil {
		t.Fatal("foreign actor listed checkers")
	}
	if _, err = ListCheckers(f.ctx, f.scope, 101, 0); err != experience.ErrInvalid {
		t.Fatal("unbounded list accepted", err)
	}
	for _, action := range []string{"version_list", "usage_list"} {
		scope := f.scope
		scope.Action = action
		tx := f.transaction(t)
		if err = authorize(f.ctx, tx, scope); err != nil {
			t.Fatal(action, err)
		}
		if err = tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
}
