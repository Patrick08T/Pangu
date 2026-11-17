//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type rolloutFixture struct {
	reservationFixture
	id, candidate string
}

func newRolloutReservation(t *testing.T) reservationFixture {
	t.Helper()
	f := reservationFixture{governanceFixture: newGovernanceFixture(t)}
	connection := environmentConnection(t, f.governanceFixture)
	node := workflow.Node{ID: "answer", Executor: "eino", Model: "offline-fixture", ConnectionVersion: connection.ID, Instruction: "回答JSON", Handoff: &workflow.HandoffContract{Objective: "回答输入", Inputs: "本次JSON", Deliverables: "JSON结果", Acceptance: "独立验证", FailureHandling: "报告失败"}}
	item, err := catalog.Create(context.Background(), f.scope.Namespace, catalog.NewObject{Kind: "agent", Name: "rollout role", Spec: catalog.Spec{Agent: &node}})
	if err != nil {
		t.Fatal(err)
	}
	base, err := catalog.Publish(context.Background(), f.scope.Namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	pool, suite := uuid.NewString(), uuid.NewString()
	err = transaction(context.Background(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), `INSERT INTO pangu.evaluation_holdout_pools(id,namespace,project_id,holdout_hash,rules_hash,max_candidates) VALUES($1,$2,$3,$4,$4,1)`, pool, f.scope.Namespace, f.scope.ProjectID, hashText(pool)); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), `INSERT INTO pangu.evaluation_suites(id,namespace,project_id,pool_id,spec,spec_hash) VALUES($1,$2,$3,$4,'{}',$5)`, suite, f.scope.Namespace, f.scope.ProjectID, pool, hashText(suite))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f.suite = &evaluation.FrozenSuite{SuiteSpec: evaluation.SuiteSpec{ID: suite}, HoldoutHash: hashText(suite)}
	f.request = CandidateRequest{ProjectID: f.scope.ProjectID, PolicyRevision: 2, AgentObjectID: item.ID, BaselineVersion: base.ID, EnvironmentID: uuid.NewString(), Entries: []EntryTarget{{Kind: "workbench", ID: "main"}}, Payload: catalog.CandidatePayload{Agent: node}}
	f.policy = reservationPolicy(f)
	f.savePolicy(t)
	return f
}

func newRolloutFixture(t *testing.T) rolloutFixture {
	t.Helper()
	f := publishedEvidenceFixture(t)
	f.service.Evidence = nil
	return f
}

func (f rolloutFixture) binding(t *testing.T) (Binding, int) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var b Binding
	var claims int
	err = conn.QueryRow(context.Background(), `SELECT entry_key,revision,adopted_version::text,stable_version::text,coalesce(intent_id::text,''),generation FROM pangu.evolution_bindings WHERE namespace=$1 AND project_id=$2 AND entry_key='workbench:main'`, f.scope.Namespace, f.scope.ProjectID).Scan(&b.EntryKey, &b.Revision, &b.AdoptedVersion, &b.StableVersion, &b.IntentID, &b.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(context.Background(), `SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1`, f.id).Scan(&claims); err != nil {
		t.Fatal(err)
	}
	return b, claims
}

func TestRolloutAdoptionKeepsStableAndClaims(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	got, err := f.service.PrepareSwitch(ctx, f.scope, f.id, 1, "workbench:main", "adopt")
	if err != nil {
		t.Fatal("adoption missing", err)
	}
	if got.VersionID != f.candidate || got.Revision != 2 {
		t.Fatal("incorrect adopted snapshot", got)
	}
	b, claims := f.binding(t)
	if b.AdoptedVersion != f.candidate || b.StableVersion != f.request.BaselineVersion || b.IntentID != f.id || claims != 2 {
		t.Fatal("adoption moved stable or released claims", b, claims)
	}
	if _, err = f.service.PrepareSwitch(ctx, f.scope, f.id, 1, "workbench:main", "adopt"); err != nil {
		t.Fatal("replay not idempotent", err)
	}
	b, _ = f.binding(t)
	if b.Revision != 2 {
		t.Fatal("replay reapplied CAS", b)
	}
	intent, err := f.service.Reconcile(ctx, f.scope, f.id, 1)
	if err != nil || intent.State != "observing" {
		t.Fatal("confirmed entries did not begin observation", intent, err)
	}
}

func TestRolloutRejectsStaleGenerationAndLivePermissionLoss(t *testing.T) {
	f := newRolloutFixture(t)
	ctx := context.Background()
	if _, err := f.service.PrepareSwitch(ctx, f.scope, f.id, 2, "workbench:main", "adopt"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale generation accepted", err)
	}
	f.policy.Revision++
	f.policy.Revoked = true
	f.savePolicy(t)
	if _, err := f.service.PrepareSwitch(ctx, f.scope, f.id, 1, "workbench:main", "adopt"); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked policy adopted", err)
	}
	b, claims := f.binding(t)
	if b.Revision != 1 || claims != 2 {
		t.Fatal("denied operation changed state", b, claims)
	}
}
