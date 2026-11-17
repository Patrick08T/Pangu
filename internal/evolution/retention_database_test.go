//go:build integration

package evolution

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRetentionWithdrawnSourceWaitsForStopBeforeClearing(t *testing.T) {
	base := newRolloutFixture(t)
	if _, err := base.service.Reconcile(context.Background(), base.scope, base.id, 1); err != nil {
		t.Fatal(err)
	}
	f := attachRetentionFixture(t, base, "withdrawn")
	stopped := false
	guard := func(_ context.Context, _ pgx.Tx, scope Scope, target RetentionTarget, phase string) (bool, error) {
		if scope.ProjectID != f.scope.ProjectID || target.IntentID != f.id || target.ExperimentID != f.experiment {
			t.Fatal("guard received foreign target", scope, target)
		}
		return phase != "stopped" || stopped, nil
	}
	out, err := f.reconcile(t, guard)
	if err != nil || !out.Due || !out.Deferred {
		t.Fatal("withdrawal not deferred", out, err)
	}
	c, v, state, claims := f.retained(t)
	if !c || !v || state != "revoked" || claims != 2 {
		t.Fatal("active body erased or claims lost", c, v, state, claims)
	}
	stopped = true
	out, err = f.reconcile(t, guard)
	if err != nil || out.Deferred || !out.CandidateCleared || out.VersionsCleared != 1 {
		t.Fatal("stopped rollback not erased", out, err)
	}
	c, v, state, claims = f.retained(t)
	if c || v || state != "revoked" || claims != 0 {
		t.Fatal("private bodies retained after safe rollback", c, v, state, claims)
	}
	again, err := f.reconcile(t, guard)
	if err != nil || again.Deferred || again.VersionsCleared != 0 {
		t.Fatal("erasure replay not idempotent", again, err)
	}
	f.assertSanitizedIntent(t)
}

func TestRetentionExplicitExpiryAuthorizesErasure(t *testing.T) {
	for _, reason := range []string{"memory_expired", "source_expired", "policy_expired"} {
		t.Run(reason, func(t *testing.T) {
			f := newRetentionFixture(t, reason)
			out, err := f.reconcile(t, retentionAllowed)
			if err != nil || !out.Due || out.Deferred {
				t.Fatal("expiry did not erase", out, err)
			}
			c, v, _, claims := f.retained(t)
			if c || v || claims != 0 {
				t.Fatal("expired private bodies retained", c, v, claims)
			}
		})
	}
}

func TestRetentionPolicyOffAndAbsentConsentNeverAuthorizeErasure(t *testing.T) {
	for _, reason := range []string{"healthy", "missing_consent", "wrong_hash"} {
		t.Run(reason, func(t *testing.T) {
			f := newRetentionFixture(t, reason)
			f.policy.Revision++
			f.policy.Enabled = false
			f.policy.Revoked = true
			f.savePolicy(t)
			out, err := f.reconcile(t, retentionAllowed)
			if err != nil || out.Due {
				t.Fatal("policyoff or unrelated source became deletion consent", out, err)
			}
			c, v, state, claims := f.retained(t)
			if !c || !v || state != "observing" || claims != 2 {
				t.Fatal("nonconsenting data changed", c, v, state, claims)
			}
		})
	}
}

func (f retentionFixture) assertSanitizedIntent(t *testing.T) {
	t.Helper()
	err := transaction(context.Background(), func(tx pgx.Tx) error {
		var empty, hashSame, identity bool
		err := tx.QueryRow(context.Background(), `SELECT request#>'{request,payload,agent}'='{}'::jsonb,request_hash=$2,experiment_id=$3 FROM pangu.evolution_intents WHERE id=$1`, f.id, f.requestHash, f.experiment).Scan(&empty, &hashSame, &identity)
		if err == nil && (!empty || !hashSame || !identity) {
			t.Fatal("legacy instruction retained or immutable identity changed", empty, hashSame, identity)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
