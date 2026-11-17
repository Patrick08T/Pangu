//go:build integration

package experience

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func TestStoreReadTxVerifiesSourcesOnCallerTransaction(t *testing.T) {
	f := newStoreFixture(t)
	version := f.create(t)
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
	checked := false
	f.store.VerifySource = func(_ context.Context, got pgx.Tx, _ Scope, _ Source) error {
		if got != tx {
			t.Fatal("source checked on a separate transaction")
		}
		checked = true
		return nil
	}
	got, err := f.store.ReadTx(f.ctx, tx, f.scope, version.ID)
	if err != nil || got.ID != version.ID || !checked {
		t.Fatalf("read through caller transaction: %+v checked=%v %v", got, checked, err)
	}
}
