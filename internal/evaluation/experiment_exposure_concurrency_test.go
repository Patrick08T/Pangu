package evaluation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

func TestExperimentDBConcurrentExposureIsSeenAfterFence(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = identity.Lock(ctx, tx, ns); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := CreateExperimentSuite(ctx, ns, spec); result <- err }()
	waitForExposureFence(t, ctx, tx, int(conn.PgConn().PID()), result)
	persistExposureFixture(t, tx, ns, spec, spec.Cases[0].SourceRunID, false, false)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; !errors.Is(err, ErrInvalid) {
		t.Fatalf("blocked suite did not observe committed exposure: %v", err)
	}
	var allocated bool
	if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evaluation_suites WHERE id=$1)`, spec.ID).Scan(&allocated); err != nil {
		t.Fatal(err)
	}
	if allocated {
		t.Fatal("concurrently exposed source left hidden allocation committed")
	}
}

func waitForExposureFence(t *testing.T, ctx context.Context, tx pgx.Tx, pid int, result <-chan error) {
	t.Helper()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var blocked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks l WHERE NOT l.granted AND $1=ANY(pg_blocking_pids(l.pid)))`, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case err := <-result:
			t.Fatalf("classification finished before source fence release: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-tick.C:
		}
	}
}
