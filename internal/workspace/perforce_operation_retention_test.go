package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Removing the local owner from Expire must not allow either retention branch
// to erase an unconfirmed Git stop's repository or its database evidence.
func TestPerforceOperationRetentionPreservesUnknownStop(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, malformed := range []bool{false, true} {
			name := map[bool]string{false: "unprepared", true: "prepared"}[prepared]
			name += map[bool]string{false: "/unknown", true: "/malformed"}[malformed]
			t.Run(name, func(t *testing.T) {
				f := newOperationRetentionFixture(t, prepared)
				before := largeRecoveryTree(t, f.run.Directory)
				sourceBefore := largeRecoveryTree(t, f.project.Source)
				marker, body := poisonOperationRetention(t, malformed)
				err := Expire(context.Background(), f.run.ID, time.Now().Add(-31*24*time.Hour))
				if !errors.Is(err, errLocalGitOperationBlocked) {
					t.Errorf("retention bypassed unknown-stop quarantine: %v", err)
				}
				if _, err := os.Stat(f.run.Directory); err != nil {
					t.Errorf("retention removed protected managed evidence: %v", err)
				} else if !reflect.DeepEqual(before, largeRecoveryTree(t, f.run.Directory)) {
					t.Error("retention changed protected managed evidence")
				}
				if !reflect.DeepEqual(sourceBefore, largeRecoveryTree(t, f.project.Source)) {
					t.Error("retention changed the bound source")
				}
				if got, err := os.ReadFile(marker); err != nil || string(got) != body {
					t.Errorf("retention removed or replaced quarantine evidence: %v", err)
				}
				assertOperationRetentionRecords(t, f, prepared)
			})
		}
	}
}

// A momentary check is insufficient: a real PostgreSQL row lock holds Expire
// before its destructive phase, while a competing local owner must be rejected.
func TestPerforceOperationRetentionOwnsWholeCleanup(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(map[bool]string{false: "unprepared", true: "prepared"}[prepared], func(t *testing.T) {
			f := newOperationRetentionFixture(t, prepared)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			tx, err := f.conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			table := map[bool]string{false: "submissions", true: "workspace_runs"}[prepared]
			if _, err := tx.Exec(ctx, "SELECT run_id FROM pangu."+table+" WHERE run_id=$1 FOR UPDATE", f.run.ID); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				defer close(done)
				done <- Expire(ctx, f.run.ID, time.Now().Add(-31*24*time.Hour))
			}()
			defer joinOperationRetention(t, cancel, tx, f.conn, done)
			waitOperationRetentionLock(t, ctx, f.conn.PgConn().PID())
			_, finish, err := beginLocalGitOperation(ctx)
			if !errors.Is(err, errLocalGitOperationBlocked) {
				t.Errorf("cleanup admitted a competing Git owner while deletion was pending: %v", err)
			}
			if finish != nil {
				if err := finish(nil); err != nil {
					t.Error(err)
				}
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatalf("healthy retention failed: %v", err)
			}
			if _, err := os.Stat(f.run.Directory); !os.IsNotExist(err) {
				t.Fatalf("healthy cleanup retained managed directory: %v", err)
			}
			if err := checkLocalGitOperation(ctx); err != nil {
				t.Fatalf("healthy retention leaked operation owner: %v", err)
			}
			if expired, err := Expired(ctx, f.run.ID); err != nil || !expired {
				t.Fatalf("healthy cleanup did not persist expiration: %v %v", expired, err)
			}
		})
	}
}

// Rerun preparation holds the worker lock before leasing its source. Expire
// must use that same order, or both callers can wait for each other forever.
func TestPerforceOperationRetentionKeepsWorkerBeforeSourceLease(t *testing.T) {
	f := newOperationRetentionFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := lockLocalWorkspace(ctx, f.conn); err != nil {
		t.Fatal(err)
	}
	defer f.conn.Exec(context.Background(), "SELECT pg_advisory_unlock_all()")
	tx, err := f.conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT run_id FROM pangu.workspace_runs WHERE run_id=$1 FOR UPDATE", f.run.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		defer close(done)
		done <- Expire(ctx, f.run.ID, time.Now().Add(-31*24*time.Hour))
	}()
	defer joinOperationRetention(t, cancel, tx, f.conn, done)
	waitOperationRetentionLock(t, ctx, f.conn.PgConn().PID())
	leaseCtx, stop := context.WithTimeout(ctx, time.Second)
	release, err := Lease(leaseCtx, f.run.ID)
	stop()
	if err != nil {
		t.Errorf("retention took source lease before acquiring worker lock: %v", err)
	}
	if release != nil {
		release()
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.conn.Exec(ctx, "SELECT pg_advisory_unlock_all()"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("healthy retention did not resume after lock release: %v", err)
	}
}

func newOperationRetentionFixture(t *testing.T, prepared bool) largeRecoveryFixture {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") != "postgresql://probe_app@127.0.0.1:6081/large_p4_frozen?sslmode=disable" || os.Getenv("PANGU_TEST_P4_PORT") != "127.0.0.1:6082" {
		t.Skip("requires the disposable large-P4 PostgreSQL/P4 fixture")
	}
	f := newLargeRecoveryFixture(t)
	if f.snapshot.WorkChange != 0 {
		t.Fatal("fixture unexpectedly owns a work changelist")
	}
	if _, err := f.conn.Exec(context.Background(), `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,created_at)
 VALUES($1,$2,$3,'submit','key','payload','{}',$4,now()-interval '31 days')`, uuid.NewString(), projectNamespace(t, f.project.ID), f.project.ID, f.run.ID); err != nil {
		t.Fatal(err)
	}
	if prepared {
		largeRecoverySQL(t, f.conn, "UPDATE pangu.workspace_runs SET created_at=now()-interval '31 days' WHERE run_id=$1", f.run.ID)
	} else {
		// Recreate initial import's DB boundary: source row exists, run row does not.
		largeRecoverySQL(t, f.conn, "DELETE FROM pangu.workspace_runs WHERE run_id=$1", f.run.ID)
		largeRecoverySQL(t, f.conn, "UPDATE pangu.perforce_sources SET commit_id='',local_commit='' WHERE run_id=$1", f.run.ID)
	}
	return f
}

func poisonOperationRetention(t *testing.T, malformed bool) (string, string) {
	t.Helper()
	ctx, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	owner := ctx.Value(localGitOperationKey{}).(*localGitOperation)
	path := filepath.Join(owner.path, owner.name)
	if malformed {
		if err := os.WriteFile(path, []byte("incomplete marker"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := finish(nil); !errors.Is(err, errLocalGitOperationBlocked) {
			t.Fatalf("malformed fixture was not quarantined: %v", err)
		}
	} else if err := finish(errLocalGitStopUnconfirmed); !errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("unknown-stop fixture lost cause: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// No process was started. Only the surrounding t.TempDir owns this marker.
	return path, string(body)
}

func assertOperationRetentionRecords(t *testing.T, f largeRecoveryFixture, prepared bool) {
	t.Helper()
	ctx := context.Background()
	var sources, records, purged int
	if err := f.conn.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM pangu.perforce_sources WHERE run_id=$1),
 (SELECT count(*) FROM pangu.retention_records WHERE run_id=$1),
 (SELECT count(*) FROM pangu.submissions WHERE run_id=$1 AND purged_at IS NOT NULL)`, f.run.ID).Scan(&sources, &records, &purged); err != nil {
		t.Fatal(err)
	}
	if sources != 1 || records != 0 || purged != 0 {
		t.Errorf("retention changed quarantine DB evidence: sources=%d records=%d purged=%d", sources, records, purged)
	}
	if prepared {
		var expired bool
		if err := f.conn.QueryRow(ctx, "SELECT expired_at IS NOT NULL OR purged_at IS NOT NULL FROM pangu.workspace_runs WHERE run_id=$1", f.run.ID).Scan(&expired); err != nil || expired {
			t.Errorf("retention permanently marked quarantined run: expired=%v err=%v", expired, err)
		}
	}
}

func joinOperationRetention(t *testing.T, cancel context.CancelFunc, tx pgx.Tx, conn *pgx.Conn, done <-chan error) {
	t.Helper()
	cancel()
	cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	_ = tx.Rollback(cleanup)
	_, _ = conn.Exec(cleanup, "SELECT pg_advisory_unlock_all()")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("retention goroutine did not stop after cancellation and lock release")
	}
}

func waitOperationRetentionLock(t *testing.T, ctx context.Context, blocker uint32) {
	t.Helper()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1::integer=ANY(pg_blocking_pids(pid)))", blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("retention did not reach the controlled DB boundary: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

// Local quarantine must not change ordinary Git retention behavior.
func TestPerforceOperationRetentionLeavesGitUnchanged(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("requires an independent test database")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "retention-git-"+uuid.NewString())
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := sourceRepository(t)
	ns, id := uuid.NewString(), uuid.NewString()
	p, err := Register(ctx, ns, Project{VCS: "git", Name: "retention control", Source: source, Branch: "main", WorkerID: WorkerID()})
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(ctx, id, p.ID, ns); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	largeRecoverySQL(t, conn, "UPDATE pangu.workspace_runs SET created_at=now()-interval '31 days' WHERE run_id=$1", id)
	marker, body := poisonOperationRetention(t, false)
	if err := Expire(ctx, id, time.Now().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("PANGU_WORKSPACE_ROOT"), id)); !os.IsNotExist(err) {
		t.Fatalf("ordinary Git retention was blocked: %v", err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != body {
		t.Fatal("ordinary Git retention changed local quarantine", err)
	}
	if _, err := os.Stat(filepath.Join(source, ".git")); err != nil {
		t.Fatal("ordinary Git source was removed", err)
	}
}
