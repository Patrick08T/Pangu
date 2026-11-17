//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func TestMemoryBenefitPassedStopsAndReleasesWithoutRolePublication(t *testing.T) {
	f := newDecisionFixture(t)
	memory := benefitMemory(t, &f)
	f.request.Kind = "memory_benefit"
	f.request.BenefitMemoryVersionID = memory.ID
	f.request.MemoryVersionIDs = []string{memory.ID}
	f.request.Payload.Agent = workflow.Node{}
	f, intent := completeDecisionFixture(t, f)
	ctx := context.Background()
	decision, err := f.service.Decide(ctx, f.scope, intent.ID, 1)
	assertPassedDecision(t, decision, err)
	err = transaction(ctx, func(tx pgx.Tx) error {
		saved, e := readIntentTx(ctx, tx, f.scope, intent.ID)
		if e != nil {
			return e
		}
		if saved.State != "stable" || saved.Publication != nil {
			t.Fatal("memory没有独立终态", saved.Intent)
		}
		var claims int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1`, intent.ID).Scan(&claims); e != nil {
			return e
		}
		if claims != 0 {
			t.Fatal("已停止评测仍占入口", claims)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err = f.service.Decide(ctx, f.scope, intent.ID, 1)
	assertPassedDecision(t, decision, err)
	if _, err = f.service.Promote(ctx, f.scope, intent.ID, 1); err == nil {
		t.Fatal("memory通过被用于角色发布")
	}
	// Canary changes delivery revision, not memory content or experiment identity.
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,evidence_id,actor_id) VALUES($1,2,'canary',$2,$3,$4)`, memory.ID, jsonBytes(experience.DeliveryPolicy{Mode: "canary", TrafficBasisPoints: 2000, TaskScope: []string{"workbench:main"}}), intent.ExperimentID, f.scope.ActorID); err != nil {
		t.Fatal(err)
	}
	scope := experience.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID}
	if err = transaction(ctx, func(tx pgx.Tx) error {
		current, e := f.service.CurrentBenefitExperimentTx(ctx, tx, scope, intent.ExperimentID)
		if e == nil && hashJSON(current) != intent.ExperimentHash {
			t.Fatal("delivery转换改变收益身份")
		}
		return e
	}); err != nil {
		t.Fatal(err)
	}
	f.policy.Revision++
	f.policy.Enabled = false
	f.savePolicy(t)
	if err = transaction(ctx, func(tx pgx.Tx) error {
		_, e := f.service.CurrentBenefitExperimentTx(ctx, tx, scope, intent.ExperimentID)
		return e
	}); !errors.Is(err, experience.ErrInvalidated) {
		t.Fatal("terminal读绕过新授权", err)
	}
}
