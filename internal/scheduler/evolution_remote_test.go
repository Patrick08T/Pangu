//go:build integration

package scheduler

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestEvolutionRemoteReceiptCannotOverridePausedEntry(t *testing.T) {
	f, _, expected, key := rolloutGuardFixture(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	id, remote := expected.Target.ID, uuid.NewString()
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		plan, _, e := evolutionPrepareRemoteTx(ctx, tx, f.scope.Namespace, key)
		if e == nil && (plan == nil || plan.Revision != int(expected.Revision)) {
			t.Fatal("remote intent did not resolve current schedule")
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := func() error {
		return evolutionTransaction(ctx, func(tx pgx.Tx) error {
			return evolutionRemoteReceiptTx(ctx, tx, f.scope.Namespace, key, expected, remote)
		})
	}
	if err = receipt(); err != nil {
		t.Fatal(err)
	}
	var ready bool
	if err = conn.QueryRow(ctx, `SELECT remote_synced FROM pangu.schedules WHERE id=$1`, id).Scan(&ready); err != nil || !ready {
		t.Fatalf("remote confirmation not committed: %v %v", ready, err)
	}
	_, err = conn.Exec(ctx, `UPDATE pangu.schedules SET paused=true,remote_synced=false WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	if err = receipt(); !errors.Is(err, evolution.ErrConflict) {
		t.Fatalf("late remote receipt overwrote pause: %v", err)
	}
	_, err = f.service.RegisterEnvironment(ctx, f.scope, evolution.Environment{ID: f.request.EnvironmentID, ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", Enabled: false, ConnectionVersion: f.request.Payload.Agent.ConnectionVersion}, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error { return evolutionRemoteAuthorityTx(ctx, tx, f.scope.Namespace, key, expected) })
	if !errors.Is(err, evolution.ErrConflict) {
		t.Fatalf("revoked environment authorized late remote effect: %v", err)
	}
}
