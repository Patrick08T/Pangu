package workspace

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEnsureRebuildKeepsSourceLeaseUntilCheckoutCompletes(t *testing.T) {
	f := newRebuildFixture(t)
	source, target := f.fork(t), uuid.NewString()
	ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
	defer cancel()
	f.sql(t, "SELECT pg_advisory_lock(hashtextextended($1,0))", "workspace:"+target)
	finished := make(chan error, 1)
	go func() {
		finished <- EnsureRebuild(ctx, target, f.project.ID, f.namespace, source)
	}()
	released := false
	defer func() {
		if !released {
			cancel()
			f.sql(t, "SELECT pg_advisory_unlock(hashtextextended($1,0))", "workspace:"+target)
			<-finished
		}
	}()
	waitRebuildSourceCheckout(t, ctx, f)
	var locked bool
	if err := f.conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1,0))", "retention:"+source).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	if locked {
		f.sql(t, "SELECT pg_advisory_unlock(hashtextextended($1,0))", "retention:"+source)
		t.Error("source retention lease was released before destination checkout")
	}
	f.sql(t, "SELECT pg_advisory_unlock(hashtextextended($1,0))", "workspace:"+target)
	released = true
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	assertRebuildSourceLeaseReleased(t, f, source)
}

func waitRebuildSourceCheckout(t *testing.T, ctx context.Context, f rebuildFixture) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := f.conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1::int = ANY(pg_blocking_pids(pid)))", f.conn.PgConn().PID()).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("rebuild did not reach destination checkout", ctx.Err())
		case <-ticker.C:
		}
	}
}
