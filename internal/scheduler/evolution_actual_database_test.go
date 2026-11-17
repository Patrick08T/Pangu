//go:build integration

package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

func TestEvolutionActualIdentityIgnoresExpectedHashesAndBindsFullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { evolutionResponse(w, true) }))
	defer server.Close()
	f, p, run := evolutionStartedFixture(t, server.URL)
	ctx := context.Background()
	if err := RunEvolutionCase(ctx, f.scope.Namespace, p.RequestID, run); err != nil {
		t.Fatal(err)
	}
	var evidence evolution.MachineResult
	if err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		var e error
		evidence, e = (EvolutionEvidence{}).ReadExecutionTx(ctx, tx, f.scope, p.RequestID)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if evidence.CompiledHash != catalog.CompiledHash(p.Definition) || evidence.RoleHash != evolutionHash(f.request.Payload.Agent) {
		t.Fatal("actual fullFlow identity differs", evidence)
	}
	p.Experiment.Candidate.RoleHash = evolutionTextHash("forged expected role")
	p.Experiment.Candidate.CompiledHash = evolutionTextHash("forged expected fullFlow")
	p.Experiment.Candidate.MemoryHash = evolutionTextHash("forged expected memory")
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		compiled, role, memory, e := evolutionActualIdentityTx(ctx, tx, f.scope, p)
		if e == nil && (compiled != evidence.CompiledHash || role != evidence.RoleHash || memory != evidence.MemoryHash) {
			t.Fatal("measurement copied expected hashes", compiled, role, memory)
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Definition.Nodes[0].Instruction += " changed actual flow"
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error { _, _, _, e := evolutionActualIdentityTx(ctx, tx, f.scope, p); return e })
	if err == nil {
		t.Fatal("actual compiled artifact no longer matches role but accepted")
	}
}

func TestEvolutionActualMemoryPlanSurvivesJSONBStorage(t *testing.T) {
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.service.PrepareCase(ctx, f.scope, evolution.CaseKey{IntentID: intent.ID, Generation: 1, Arm: "candidate", CaseID: f.caseID(), Repeat: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Storage-only fixture: it must not establish efficacy or execute this unapproved memory.
	p.Memories = append(p.Memories, actualMemoryFixture())
	if err = evolutionPersistPlan(ctx, f.scope, *p); err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, _, e := evolutionReadPlanTx(ctx, tx, f.scope.Namespace, p.RequestID)
		return e
	})
	if err != nil {
		t.Fatal("canonical memory content changed envelope hash after JSONB", err)
	}
}
