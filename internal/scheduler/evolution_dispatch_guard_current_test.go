//go:build integration

package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestEvolutionDispatchFreshPrepareRejectsWithdrawnMemoryOrSource(t *testing.T) {
	for name, update := range map[string]string{
		"erased_body":      `DELETE FROM pangu.experience_bodies WHERE version_id IN (SELECT (i.request->'request'->>'benefit_memory_version_id')::uuid FROM pangu.evolution_intents i JOIN pangu.evolution_executions e ON e.intent_id=i.id WHERE e.request_id=$1)`,
		"withdrawn_source": `UPDATE pangu.learning_source_consents c SET revoked_at=now() FROM pangu.submissions s WHERE s.id=$1 AND c.namespace=s.namespace AND c.project_id=s.project_id`,
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := actualBenefitFixture(t, "http://127.0.0.1:1")
			p := evolutionPreparePending(t, f)
			evolutionGuardRejects(t, f, p, update)
		})
	}
}

func TestEvolutionDispatchRechecksCurrentAuthorityAndPolicy(t *testing.T) {
	for name, update := range map[string]string{
		"disabled_actor":  `UPDATE pangu.users SET disabled=true WHERE id IN (SELECT created_by FROM pangu.submissions WHERE id=$1)`,
		"revoked_policy":  `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT p.namespace,p.project_id,p.revision+1,p.actor_id,p.policy||jsonb_build_object('revision',p.revision+1,'revoked',true) FROM pangu.experience_policies p JOIN pangu.submissions s ON s.namespace=p.namespace AND s.project_id=p.project_id WHERE s.id=$1`,
		"expired_request": `UPDATE pangu.submissions SET created_at=now()-interval '90 days' WHERE id=$1`,
	} {
		t.Run(name, func(t *testing.T) { f, p := evolutionPendingFixture(t); evolutionGuardRejects(t, f, p, update) })
	}
}

func TestEvolutionDispatchCommitsBeforeRPCAndRuntimeRejectsLateRevocation(t *testing.T) {
	var provider atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { provider.Add(1); evolutionResponse(w, true) }))
	defer server.Close()
	f := newEvolutionFixture(t, server.URL)
	p := evolutionPreparePending(t, f)
	ctx := context.Background()
	run, calls := uuid.NewString(), 0
	record, err := submission.Dispatch(ctx, f.scope.Namespace, p.RequestID, func(ctx context.Context, item *submission.Record) (string, error) {
		return evolutionDispatch(ctx, f.scope.Namespace, item.ID, func(ctx context.Context, id string) (string, error) {
			calls++
			if id != p.RequestID {
				t.Fatal("engine did not receive exact durable request identity")
			}
			err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout='150ms'`); err != nil {
					return err
				}
				if err := identity.Lock(ctx, tx, f.scope.Namespace); err != nil {
					return err
				}
				if err := experience.Lock(ctx, tx, f.scope.Namespace, f.scope.ProjectID); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.scope.ActorID)
				return err
			})
			return run, err
		})
	})
	if err != nil || calls != 1 || record.RunID != run {
		t.Fatalf("dispatch kept SQL locks across RPC: calls=%d %v", calls, err)
	}
	if err = RunEvolutionCase(ctx, f.scope.Namespace, p.RequestID, run); err == nil {
		t.Fatal("runtime ignored revocation after preflight")
	}
	if provider.Load() != 0 {
		t.Fatalf("revoked runtime called provider: %d", provider.Load())
	}
}
