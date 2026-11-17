//go:build integration

package experience

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestStoreEmptyFreezeDoesNotGrantAnonymousAccess(t *testing.T) {
	f := newStoreFixture(t)
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	tx, err := conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	anonymous := f.scope
	anonymous.ActorID = ""
	mark, err := (Store{}).FreezeEmptyTx(f.ctx, tx, anonymous, uuid.NewString(), 1, "business")
	if err != nil || mark.Hash == "" || len(mark.Versions) != 0 {
		t.Fatalf("auditable disabled empty freeze: %+v %v", mark, err)
	}
	if _, err = (Store{}).FreezeEmptyTx(f.ctx, tx, anonymous, uuid.NewString(), 1, "unknown"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown purpose: %v", err)
	}
}

func TestStoreEmptyFreezeRejectsEnabledAnonymousBusiness(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	tx, err := conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	anonymous := f.scope
	anonymous.ActorID = ""
	if _, err = (Store{}).FreezeEmptyTx(f.ctx, tx, anonymous, uuid.NewString(), 1, "business"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("enabled business silently downgraded: %v", err)
	}
	for _, purpose := range []string{"learning", "evaluation"} {
		mark, err := (Store{}).FreezeEmptyTx(f.ctx, tx, anonymous, uuid.NewString(), 1, purpose)
		if err != nil || mark.Hash == "" || len(mark.Versions) != 0 {
			t.Fatalf("internal purpose not empty: %+v %v", mark, err)
		}
	}
}
