//go:build integration

package learning

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Fault only the selected real query's context, after identity.Lock succeeds.
// pgx still executes and reports the actual I/O error; no row results are mocked.
type authorityErrorTrace struct {
	query string
	ctx   context.Context
	err   error
}

func (f *authorityErrorTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, f.query) {
		return f.ctx
	}
	return ctx
}

func (f *authorityErrorTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if ctx == f.ctx {
		f.err = data.Err
	}
}

func authorityErrorTransaction(t *testing.T, trace *authorityErrorTrace) pgx.Tx {
	t.Helper()
	config, err := pgx.ParseConfig(os.Getenv("PANGU_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid isolated test database configuration")
	}
	config.Tracer = trace
	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		t.Fatal("cannot connect to isolated test database")
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback(context.Background()) })
	return tx
}

func TestLearningAuthorizationPreservesQueryContextErrors(t *testing.T) {
	for _, query := range []string{"SELECT u.is_admin", "SELECT scopes FROM"} {
		for _, want := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(query+"/"+want.Error(), func(t *testing.T) {
				f := newLearningFixture(t)
				ctx := f.ctx
				if query == "SELECT scopes FROM" {
					ctx = f.token(t, []string{"read"})
				}
				scope := f.scope
				scope.Action = "policy_read"
				failed, cancel := context.WithCancel(ctx)
				if want == context.DeadlineExceeded {
					cancel()
					failed, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				}
				cancel()
				trace := &authorityErrorTrace{query: query, ctx: failed}
				err := f.store.Authorize(ctx, authorityErrorTransaction(t, trace), scope)
				if !errors.Is(err, want) || err != trace.err || errors.Is(err, experience.ErrForbidden) {
					t.Fatalf("query error not preserved unchanged: got %v, query %v, want %v", err, trace.err, want)
				}
			})
		}
	}
}

func TestLearningAuthorizationPreservesTokenDatabaseErrors(t *testing.T) {
	f := newLearningFixture(t)
	ctx := f.token(t, []string{"read"})
	scope := f.scope
	scope.Action = "policy_read"
	tx := f.transaction(t)
	if err := f.store.Authorize(ctx, tx, scope); err != nil {
		t.Fatal("valid token denied", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	tx = f.transaction(t)
	if _, err := tx.Exec(ctx, "SET TRANSACTION READ ONLY"); err != nil {
		t.Fatal(err)
	}
	err := f.store.Authorize(ctx, tx, scope)
	var sqlErr *pgconn.PgError
	if errors.Is(err, experience.ErrForbidden) || !errors.As(err, &sqlErr) || sqlErr.Code != "25006" {
		t.Fatalf("token query failure lost its database cause: %v", err)
	}
}
