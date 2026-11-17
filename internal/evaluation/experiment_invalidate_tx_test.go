package evaluation

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestExperimentDBInvalidateTxRollsBackAndPreservesWrapper(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	ctx := context.Background()
	suite, err := CreateExperimentSuite(ctx, ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperiment(ctx, ns, dbExperimentSpec(suite))
	if err != nil {
		t.Fatal(err)
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
	defer tx.Rollback(ctx)
	if err = InvalidateExperimentTx(ctx, tx, ns, e.ID, e.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadExperimentTx(ctx, tx, ns, suite.ProjectID, e.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("tx未执行失效CAS", err)
	}
	if _, err = tx.Exec(ctx, `SELECT 1`); err != nil {
		t.Fatal("内部提交调用者事务", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadExperiment(ctx, ns, suite.ProjectID, e.ID); err != nil {
		t.Fatal("外部回滚没有恢复实验", err)
	}
	if err = InvalidateExperiment(ctx, ns, e.ID, e.Generation); err != nil {
		t.Fatal(err)
	}
	if err = InvalidateExperiment(ctx, ns, e.ID, e.Generation); !errors.Is(err, ErrConflict) {
		t.Fatal("旧API代际CAS失效", err)
	}
}

func TestExperimentDBInvalidateTxRetainsRowFenceUntilOuterCompletion(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	ctx := context.Background()
	suite, err := CreateExperimentSuite(ctx, ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperiment(ctx, ns, dbExperimentSpec(suite))
	if err != nil {
		t.Fatal(err)
	}
	first, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close(ctx)
	tx, err := first.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = InvalidateExperimentTx(ctx, tx, ns, e.ID, e.Generation); err != nil {
		t.Fatal(err)
	}
	second, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close(ctx)
	other, err := second.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(ctx)
	if _, err = other.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); err != nil {
		t.Fatal(err)
	}
	err = InvalidateExperimentTx(ctx, other, ns, e.ID, e.Generation)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "55P03" {
		t.Fatal("失效未持有同一实验行锁", err)
	}
}
