//go:build integration

package learning

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMemoryReconcilePreservesUnknownAndSQLFailures(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	policy, _ := json.Marshal(version.Canary)
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id,evidence_id)
  VALUES($1,3,'canary',$2,$3,$4)`, version.ID, policy, f.actor.ID, version.ID)
	unknown := errors.New("temporary plan reader unavailable")
	reader := func(context.Context, pgx.Tx, experience.Scope, string) (evaluation.ExperimentSpec, error) {
		return evaluation.ExperimentSpec{}, unknown
	}
	paused, err := CheckMemoryObservation(context.Background(), f.scope, version.ID, reader)
	if paused || !errors.Is(err, unknown) {
		t.Fatal("unknown failure converted into pause", paused, err)
	}
	reader = func(ctx context.Context, tx pgx.Tx, _ experience.Scope, _ string) (evaluation.ExperimentSpec, error) {
		_, err := tx.Exec(ctx, `SELECT missing_column_for_learning_guard FROM pangu.projects`)
		return evaluation.ExperimentSpec{}, err
	}
	paused, err = CheckMemoryObservation(context.Background(), f.scope, version.ID, reader)
	var sqlErr *pgconn.PgError
	if paused || !errors.As(err, &sqlErr) || sqlErr.Code != "42703" {
		t.Fatal("SQL failure masked", paused, err)
	}
	current, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil || current.Delivery != "canary" || current.DeliveryRevision != 3 {
		t.Fatal("failed check mutated delivery", current, err)
	}
}
