//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
)

func TestEvolutionCancelRouteKeepsUnknownThenFinishesConfirmedStop(t *testing.T) {
	f, p, run := evolutionStartedFixture(t, "http://127.0.0.1:1")
	ctx := context.Background()
	current, err := f.service.CancelEvaluation(ctx, f.scope, p.Key.IntentID, p.Key.Generation, "stop evaluation")
	if err != nil || current.State != "reconciling" {
		t.Fatal(current, err)
	}
	b := Backend{}
	if err = b.evolutionAdvanceIntent(ctx, f.scope, current.ID, current.Generation, current.State); err != nil {
		t.Fatal("prepublication routed to rollout", err)
	}
	assertEvolutionCancelState(t, f, current.ID, "reconciling", true)
	if err = evolutionFinishCase(ctx, p.RequestID, run, "failed", nil); err != nil {
		t.Fatal(err)
	}
	if err = b.evolutionAdvanceIntent(ctx, f.scope, current.ID, current.Generation, "reconciling"); err != nil {
		t.Fatal(err)
	}
	assertEvolutionCancelState(t, f, current.ID, "insufficient", false)
	if err = b.evolutionAdvanceIntent(ctx, f.scope, current.ID, current.Generation+1, "reconciling"); err == nil {
		t.Fatal("missing generation confused with absent publication")
	}
}

func assertEvolutionCancelState(t *testing.T, f evolutionFixture, id, state string, held bool) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var actual string
	var claims int
	if err = conn.QueryRow(ctx, `SELECT state,(SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=i.id) FROM pangu.evolution_intents i WHERE id=$1`, id).Scan(&actual, &claims); err != nil {
		t.Fatal(err)
	}
	if actual != state || (claims > 0) != held {
		t.Fatal("stop/claim boundary wrong", actual, claims)
	}
}
