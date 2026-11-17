//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRetentionUnknownGuardFailureRollsBackAllChanges(t *testing.T) {
	for _, problem := range []string{"sql", "cancelled", "deadline"} {
		t.Run(problem, func(t *testing.T) {
			f := newRetentionFixture(t, "withdrawn")
			guard := func(ctx context.Context, tx pgx.Tx, _ Scope, _ RetentionTarget, phase string) (bool, error) {
				if phase == "stopped" {
					return true, nil
				}
				if problem == "cancelled" {
					return false, context.Canceled
				}
				if problem == "deadline" {
					return false, context.DeadlineExceeded
				}
				_, err := tx.Exec(ctx, `SELECT 1/0`)
				return false, err
			}
			_, err := f.reconcile(t, guard)
			var sqlErr *pgconn.PgError
			if problem == "sql" && (!errors.As(err, &sqlErr) || sqlErr.Code != "22012") {
				t.Fatal("SQL failure lost", err)
			}
			if problem == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if problem == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			c, v, state, claims := f.retained(t)
			if !c || !v || state != "observing" || claims != 2 {
				t.Fatal("unknown outcome committed erasure/revoke", c, v, state, claims)
			}
		})
	}
}

func TestRetentionMissingGuardRevokesButDoesNotErase(t *testing.T) {
	f := newRetentionFixture(t, "withdrawn")
	out, err := f.reconcile(t, nil)
	if err != nil || !out.Deferred || out.Reason == "" {
		t.Fatal("missing guard treated as completed", out, err)
	}
	c, v, state, claims := f.retained(t)
	if !c || !v || state != "revoked" || claims != 2 {
		t.Fatal("missing guard erased active body", c, v, state, claims)
	}
}

func TestRetentionStableInUseRevokesWithoutTakingClaims(t *testing.T) {
	f := newRetentionFixture(t, "withdrawn")
	f.exec(t, `UPDATE pangu.evolution_intents SET state='stable' WHERE id=$1`, f.id)
	f.exec(t, `UPDATE pangu.catalog_version_state SET state='stable',revision=revision+1 WHERE version_id=$1`, f.candidate)
	f.exec(t, `UPDATE pangu.evolution_bindings SET adopted_version=$3,stable_version=$3 WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID, f.candidate)
	f.exec(t, `DELETE FROM pangu.evolution_target_claims WHERE intent_id=$1`, f.id)
	out, err := f.reconcile(t, retentionAllowed)
	if err != nil || !out.Deferred {
		t.Fatal("live stable binding did not defer", out, err)
	}
	c, v, state, claims := f.retained(t)
	if !c || !v || state != "revoked" || claims != 0 {
		t.Fatal("stable revoked erasure took old claims", c, v, state, claims)
	}
	binding, _ := f.binding(t)
	if binding.AdoptedVersion != f.candidate || binding.StableVersion != f.candidate {
		t.Fatal("retention replaced live stable entry", binding)
	}
	f.exec(t, `UPDATE pangu.evolution_bindings SET adopted_version=$3,stable_version=$3,revision=revision+1 WHERE namespace=$1 AND project_id=$2`, f.scope.Namespace, f.scope.ProjectID, f.request.BaselineVersion)
	out, err = f.reconcile(t, retentionAllowed)
	if err != nil || out.Deferred || out.VersionsCleared != 1 {
		t.Fatal("detached stopped stable not cleared", out, err)
	}
}

func TestRetentionFrozenPolicyRevisionMustMatchStoredIdentity(t *testing.T) {
	f := newRetentionFixture(t, "withdrawn")
	next := f.policy.Revision + 1
	f.exec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,$3,$4,$5)`, f.scope.Namespace, f.scope.ProjectID, next, f.scope.ActorID, jsonBytes(f.policy))
	f.exec(t, `UPDATE pangu.evolution_intents SET policy_revision=$2 WHERE id=$1`, f.id, next)
	if _, err := f.reconcile(t, retentionAllowed); !errors.Is(err, ErrForbidden) {
		t.Fatal("mismatched frozen policy authorized erasure", err)
	}
	c, v, state, claims := f.retained(t)
	if !c || !v || state != "observing" || claims != 2 {
		t.Fatal("corrupt policy committed changes", c, v, state, claims)
	}
}

func TestRetentionScopeAndGenerationFailClosed(t *testing.T) {
	f := newRetentionFixture(t, "withdrawn")
	scope := f.scope
	scope.ActorID = ""
	if _, err := f.service.ReconcileRetention(context.Background(), scope, f.id, 2, retentionAllowed); !errors.Is(err, ErrConflict) {
		t.Fatal("stale retention accepted", err)
	}
	scope.ProjectID = uuid.NewString()
	if _, err := f.service.ReconcileRetention(context.Background(), scope, f.id, 1, retentionAllowed); err == nil {
		t.Fatal("cross-project retention accepted")
	}
	if _, err := f.service.ReconcileRetention(context.Background(), f.scope, f.id, 1, retentionAllowed); !errors.Is(err, ErrForbidden) {
		t.Fatal("user called server-only retention", err)
	}
	c, v, _, claims := f.retained(t)
	if !c || !v || claims != 2 {
		t.Fatal("foreign/old request mutated bodies", c, v, claims)
	}
}
