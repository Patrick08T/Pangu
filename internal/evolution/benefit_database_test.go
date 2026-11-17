//go:build integration

package evolution

import (
	"context"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func benefitMemory(t *testing.T, f *reservationFixture) experience.Version {
	t.Helper()
	return benefitScopedMemory(t, f, experience.Applicability{Roles: []string{f.request.AgentObjectID}})
}

func benefitScopedMemory(t *testing.T, f *reservationFixture, applicability experience.Applicability) experience.Version {
	t.Helper()
	ctx := context.Background()
	f.service.Memory = experience.Store{Authorize: func(context.Context, pgx.Tx, experience.Scope) error { return nil }, VerifySource: func(context.Context, pgx.Tx, experience.Scope, experience.Source) error { return nil }}
	version := experience.Version{Revision: 1, ProjectID: f.scope.ProjectID, ExpiresAt: time.Now().Add(time.Hour), Content: experience.Content{Kind: "practice", Observation: "遗漏JSON字段", Practice: "逐项核验必填字段", Conditions: "JSON任务", Counterexamples: "不适用于纯文本", Applicability: applicability}, Sources: []experience.Source{{ID: uuid.NewString(), Kind: "artifact", RunID: uuid.NewString(), NodeID: "answer", ProjectID: f.scope.ProjectID, Hash: hashText("retained business source")}}}
	saved, err := f.service.Memory.Create(ctx, experience.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID}, version)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `UPDATE pangu.experience_states SET validity='active' WHERE version_id=$1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	return saved
}

func TestMemoryBenefitPlanFreezesSameRoleAndDifferentActualMemories(t *testing.T) {
	f := newReservationFixture(t)
	memory := benefitMemory(t, &f)
	f.service.Evidence = &executionEvidence{}
	f.request.Kind = "memory_benefit"
	f.request.BenefitMemoryVersionID = memory.ID
	f.request.MemoryVersionIDs = []string{memory.ID}
	f.request.Payload.Agent = workflow.Node{}
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal("受控memory plan未预留", err)
	}
	caseID := ""
	for _, c := range f.suite.Cases {
		if c.Partition != "development" {
			caseID = c.ID
			break
		}
	}
	key := CaseKey{IntentID: intent.ID, Generation: 1, Arm: "baseline", CaseID: caseID, Repeat: 1}
	base, err := f.service.PrepareCase(ctx, f.scope, key)
	if err != nil {
		t.Fatal(err)
	}
	key.Arm = "candidate"
	candidate, err := f.service.PrepareCase(ctx, f.scope, key)
	if err != nil {
		t.Fatal(err)
	}
	if base.Experiment.Kind != "memory_benefit" || base.Experiment.Baseline.MemoryHash != evaluation.EmptyMemoryHash || len(base.Memories) != 0 || len(candidate.Memories) != 1 || candidate.Experiment.CandidateHash != memory.BodyHash || catalog.CompiledHash(base.Definition) != catalog.CompiledHash(candidate.Definition) {
		t.Fatal("memory实验混入角色变化或错误注入", base, candidate)
	}
	if _, err = f.service.Promote(ctx, f.scope, intent.ID, 1); err == nil {
		t.Fatal("memory证据不得发布角色")
	}
	err = transaction(ctx, func(tx pgx.Tx) error {
		current, e := f.service.CurrentBenefitExperimentTx(ctx, tx, experience.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID}, intent.ExperimentID)
		if e == nil && hashJSON(current) != intent.ExperimentHash {
			t.Fatal("current不是实时重建")
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	f.service.Memory.VerifySource = func(context.Context, pgx.Tx, experience.Scope, experience.Source) error {
		return experience.ErrInvalidated
	}
	if err = transaction(ctx, func(tx pgx.Tx) error {
		_, e := f.service.CurrentBenefitExperimentTx(ctx, tx, experience.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID}, intent.ExperimentID)
		return e
	}); err == nil {
		t.Fatal("来源失效仍复用冻结current")
	}
}
