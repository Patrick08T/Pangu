//go:build integration

package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestEvolutionRolloutGuardSourceLossBlocksRemoteDispatchAndReceipt(t *testing.T) {
	for _, phase := range []string{"dispatch", "receipt"} {
		t.Run(phase, func(t *testing.T) {
			f, _, expected, key := rolloutGuardFixture(t)
			ctx := context.Background()
			if phase == "receipt" {
				if err := rolloutGuardDispatch(ctx, f, key); err != nil {
					t.Fatal(err)
				}
			}
			rolloutGuardExec(t, `UPDATE pangu.evaluation_cases SET archived=true WHERE id=$1`, f.suite.Cases[0].ID)
			var err error
			if phase == "dispatch" {
				err = rolloutGuardDispatch(ctx, f, key)
			} else {
				err = rolloutGuardReceipt(ctx, f, key, expected)
			}
			if err == nil {
				t.Fatal("remote effect accepted invalid current suite source", phase)
			}
			rolloutGuardRetained(t, f, expected, key)
		})
	}
}

func rolloutGuardExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if err := evolutionTransaction(context.Background(), func(tx pgx.Tx) error { _, err := tx.Exec(context.Background(), sql, args...); return err }); err != nil {
		t.Fatal(err)
	}
}

func rolloutGuardDispatch(ctx context.Context, f evolutionFixture, key string) error {
	return evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, _, err := evolutionPrepareRemoteTx(ctx, tx, f.scope.Namespace, key)
		return err
	})
}
func rolloutGuardReceipt(ctx context.Context, f evolutionFixture, key string, expected evolution.EntrySnapshot) error {
	return evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if err := evolutionRemoteAuthorityTx(ctx, tx, f.scope.Namespace, key, expected); err != nil {
			return err
		}
		return evolutionRemoteReceiptTx(ctx, tx, f.scope.Namespace, key, expected, uuid.NewString())
	})
}
func rolloutGuardRetained(t *testing.T, f evolutionFixture, expected evolution.EntrySnapshot, key string) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var synced, body bool
	var state, status string
	var claims int
	err = conn.QueryRow(context.Background(), `SELECT s.remote_synced,b.spec IS NOT NULL,v.state,r.status,(SELECT count(*) FROM pangu.evolution_target_claims WHERE namespace=$1 AND project_id=$2) FROM pangu.schedules s JOIN pangu.catalog_version_bodies b ON b.version_id=$4 JOIN pangu.catalog_version_state v ON v.version_id=b.version_id JOIN pangu.evolution_remote_intents r ON r.effect_key=$5 AND r.namespace=$1 WHERE s.namespace=$1 AND s.project_id=$2 AND s.id=$3`, f.scope.Namespace, f.scope.ProjectID, expected.Target.ID, expected.VersionID, key).Scan(&synced, &body, &state, &status, &claims)
	if err != nil || synced || !body || state != "observing" || status == "confirmed" || claims != 4 {
		t.Fatal("denied remote lost uncertainty/claims/body", synced, body, state, status, claims, err)
	}
}

func TestEvolutionRolloutGuardDisabledEvaluatorIsNotInvalidEvidence(t *testing.T) {
	f, intent, expected, key := rolloutGuardFixture(t)
	ctx := context.Background()
	_, err := f.service.RegisterEnvironment(ctx, f.scope, evolution.Environment{ID: f.request.EnvironmentID, ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", Enabled: false, ConnectionVersion: f.request.Payload.Agent.ConnectionVersion}, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error { return f.service.PublishedEvidenceTx(ctx, tx, f.scope, intent.ID, 1) })
	if err != nil {
		t.Fatal("executor pause invalidated published evidence", err)
	}
	if err = rolloutGuardDispatch(ctx, f, key); !errors.Is(err, evolution.ErrConflict) {
		t.Fatal("existing remote execution pause was bypassed", err)
	}
	rolloutGuardRetained(t, f, expected, key)
}

func TestEvolutionRolloutGuardInvalidSourceStillAllowsCompensation(t *testing.T) {
	f, intent, expected, key := rolloutGuardFixture(t)
	ctx := context.Background()
	if err := rolloutGuardDispatch(ctx, f, key); err != nil {
		t.Fatal(err)
	}
	if err := rolloutGuardReceipt(ctx, f, key, expected); err != nil {
		t.Fatal(err)
	}
	if out, err := f.service.Reconcile(ctx, f.scope, intent.ID, 1); err != nil || out.State != "observing" {
		t.Fatal("adopt", out, err)
	}
	rolloutGuardExec(t, `UPDATE pangu.evaluation_cases SET archived=true WHERE id=$1`, f.suite.Cases[0].ID)
	if _, err := f.service.Reconcile(ctx, f.scope, intent.ID, 1); err != nil {
		t.Fatal("begin compensation", err)
	}
	back, err := f.service.PrepareSwitch(ctx, f.scope, intent.ID, 1, expected.Target.Key(), "rollback")
	if err != nil || back == nil || back.VersionID != f.request.Payload.FlowVersion {
		t.Fatal("rollback snapshot", back, err)
	}
	rollback := intent.ID + ":1:" + expected.Target.Key() + ":rollback"
	if err = rolloutGuardDispatch(ctx, f, rollback); err != nil {
		t.Fatal("source invalidity blocked compensating dispatch", err)
	}
	if err = rolloutGuardReceipt(ctx, f, rollback, *back); err != nil {
		t.Fatal("source invalidity blocked compensating confirmation", err)
	}
	out, err := f.service.Reconcile(ctx, f.scope, intent.ID, 1)
	if err != nil || out.State != "insufficient" {
		t.Fatal("compensation did not finish", out, err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var claims int
	var retained bool
	var state string
	err = conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1),b.spec IS NOT NULL,s.state FROM pangu.catalog_version_bodies b JOIN pangu.catalog_version_state s ON s.version_id=b.version_id WHERE b.version_id=$2`, intent.ID, intent.Publication.AgentID).Scan(&claims, &retained, &state)
	if err != nil || claims != 0 || !retained || state != "revoked" {
		t.Fatal("compensation erased body or leaked claims", claims, retained, state, err)
	}
}

func TestEvolutionRolloutGuardUnknownReadCannotConfirmRemote(t *testing.T) {
	f, _, expected, key := rolloutGuardFixture(t)
	ctx := context.Background()
	if err := rolloutGuardDispatch(ctx, f, key); err != nil {
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
	if _, err = tx.Exec(ctx, `SELECT 1 FROM pangu.evaluation_cases WHERE id=$1 FOR UPDATE`, f.suite.Cases[0].ID); err != nil {
		t.Fatal(err)
	}
	limited, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err = rolloutGuardReceipt(limited, f, key, expected); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unknown source read confirmed remote", err)
	}
	rolloutGuardRetained(t, f, expected, key)
}
