//go:build integration

package evolution

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRetentionUnpublishedCandidateWaitsThenScrubsLegacyInstruction(t *testing.T) {
	base := newReservationFixture(t)
	intent, err := base.service.ReserveCandidate(context.Background(), base.scope, base.request)
	if err != nil {
		t.Fatal(err)
	}
	f := retentionFixture{rolloutFixture: rolloutFixture{reservationFixture: base, id: intent.ID}, experiment: intent.ExperimentID}
	f.exec(t, `UPDATE pangu.evolution_intents SET created_at=clock_timestamp()-interval '100 days',request=jsonb_set(request,'{request,payload,agent}',$2) WHERE id=$1`, f.id, jsonBytes(f.request.Payload.Agent))
	out, err := f.reconcile(t, nil)
	if err != nil || !out.Due || !out.Deferred || out.State != "reconciling" {
		t.Fatal("unpublished missing stop proof was not fenced", out, err)
	}
	var claims int
	var present bool
	err = transaction(context.Background(), func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1),payload IS NOT NULL FROM pangu.catalog_experiment_candidates WHERE experiment_id=$2`, f.id, f.experiment).Scan(&claims, &present)
	})
	if err != nil || claims == 0 || !present {
		t.Fatal("unpublished unknown stop lost body/claims", claims, present, err)
	}
	out, err = f.reconcile(t, retentionAllowed)
	if err != nil || out.Deferred || !out.CandidateCleared || out.VersionsCleared != 0 {
		t.Fatal("prepublication erasure reporting incomplete", out, err)
	}
	assertCancelledLedger(t, base, intent)
	err = transaction(context.Background(), func(tx pgx.Tx) error {
		var scrubbed, unchanged bool
		err := tx.QueryRow(context.Background(), `SELECT request#>'{request,payload,agent}'='{}'::jsonb,request_hash=$2 FROM pangu.evolution_intents WHERE id=$1`, f.id, intent.RequestHash).Scan(&scrubbed, &unchanged)
		if err == nil && (!scrubbed || !unchanged) {
			t.Fatal("legacy payload not erased safely", scrubbed, unchanged)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
