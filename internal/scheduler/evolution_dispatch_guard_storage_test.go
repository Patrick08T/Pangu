//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestEvolutionDispatchMissingPlanAndStorageErrorsAreNotApproval(t *testing.T) {
	f, p := evolutionPendingFixture(t)
	ctx := context.Background()
	calls := 0
	_, err := evolutionDispatch(ctx, f.scope.Namespace, uuid.NewString(), func(context.Context, string) (string, error) { calls++; return uuid.NewString(), nil })
	if !errors.Is(err, pgx.ErrNoRows) || calls != 0 {
		t.Fatalf("missing plan approved: %d %v", calls, err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = evolutionDispatchRequestTx(ctx, tx, f.scope, p)
	if !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("unknown storage error was normalized: %v", err)
	}
}

func TestEvolutionDispatchSnapshotComparisonPreservesExactNumbers(t *testing.T) {
	left := []byte(`{"budget":9007199254740993,"purpose":"evaluation"}`)
	if !evolutionDispatchSameJSON(left, []byte(`{"purpose":"evaluation", "budget":9007199254740993}`)) {
		t.Fatal("JSONB ordering changed semantic identity")
	}
	for _, raw := range []string{
		`{"purpose":"evaluation","budget":9007199254740992}`,
		`{"purpose":"evaluation","budget":9007199254740993,"ApprovalOrigin":"hidden"}`,
		`{"Purpose":"evaluation","budget":9007199254740993}`,
	} {
		if evolutionDispatchSameJSON(left, []byte(raw)) {
			t.Fatal("changed or extra actual snapshot accepted", raw)
		}
	}
}
