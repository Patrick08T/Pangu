//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func actualBenefitSource(t *testing.T, f evolutionFixture) experience.Source {
	t.Helper()
	ctx := context.Background()
	run := evolutionBusinessFixture(t, f, f.request.Payload.FlowVersion, "independent memory source "+uuid.NewString(), "accepted")
	source := experience.Source{ID: uuid.NewString(), Kind: "report", RunID: run, NodeID: "research", ProjectID: f.scope.ProjectID}
	node := uuid.NewString()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `UPDATE pangu.run_history SET summary=summary||jsonb_build_object('nodes',jsonb_build_array(jsonb_build_object('id',$2::text,'node_id','research'))) WHERE run_id=$1`, run, node)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence) VALUES($1,$2,$3,'retained source',$4,'{}','[]')`, source.ID, run, node, []byte(`{"result":{"valid":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	if err = conn.QueryRow(ctx, `SELECT body FROM pangu.research_reports WHERE id=$1`, source.ID).Scan(&body); err != nil {
		t.Fatal(err)
	}
	source.Hash = evolutionTextHash(string(body))
	scope := experience.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID}
	if err = learning.GrantSource(identity.WithActor(ctx, identity.Actor{ID: f.scope.ActorID, Namespace: f.scope.Namespace}), scope, source, time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	return source
}

func actualBenefitFixture(t *testing.T, url string) (evolutionFixture, experience.Version) {
	t.Helper()
	f := newEvolutionFixture(t, url)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) SELECT namespace,project_id,2,actor_id,policy||'{"revision":2,"read_enabled":true}'::jsonb FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=1`, f.scope.Namespace, f.scope.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	f.request.PolicyRevision = 2
	f.service.Memory = learning.NewStore()
	source := actualBenefitSource(t, f)
	var content experience.Content
	if err = json.Unmarshal(actualMemoryFixture().Content, &content); err != nil {
		t.Fatal(err)
	}
	content.Applicability.Roles = []string{f.request.AgentObjectID}
	version, err := f.service.Memory.Create(ctx, experience.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID}, experience.Version{Revision: 1, ProjectID: f.scope.ProjectID, ExpiresAt: time.Now().Add(20 * time.Minute), Content: content, Sources: []experience.Source{source}})
	if err != nil {
		t.Fatal(err)
	}
	// Eligibility fixture only; the test below uses the actual source, consent, runtime, and evidence readers.
	if _, err = conn.Exec(ctx, `UPDATE pangu.experience_states SET validity='active' WHERE version_id=$1`, version.ID); err != nil {
		t.Fatal(err)
	}
	f.request.Kind = "memory_benefit"
	f.request.BenefitMemoryVersionID = version.ID
	f.request.MemoryVersionIDs = []string{version.ID}
	f.request.Payload.Agent = workflow.Node{}
	return f, version
}

func TestEvolutionActualBenefitRunsBothArmsWithMeasuredMemoryIdentity(t *testing.T) {
	var calls, withMemory atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls.Add(1)
		if strings.Contains(string(body), "Check required keys") {
			withMemory.Add(1)
		}
		evolutionResponse(w, true)
	}))
	defer server.Close()
	f, memory := actualBenefitFixture(t, server.URL)
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	var results []evolution.MachineResult
	for _, arm := range []string{"baseline", "candidate"} {
		p, err := f.service.PrepareCase(ctx, f.scope, evolution.CaseKey{IntentID: intent.ID, Generation: 1, Arm: arm, CaseID: f.caseID(), Repeat: 1})
		if err != nil {
			t.Fatal(err)
		}
		if err = evolutionPersistPlan(ctx, f.scope, *p); err != nil {
			t.Fatal(err)
		}
		if err = RunEvolutionCase(ctx, f.scope.Namespace, p.RequestID, uuid.NewString()); err != nil {
			t.Fatal(err)
		}
		var result evolution.MachineResult
		if err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
			var e error
			result, e = (EvolutionEvidence{}).ReadExecutionTx(ctx, tx, f.scope, p.RequestID)
			return e
		}); err != nil {
			t.Fatal(err)
		}
		if err = f.service.RecordCaseResult(ctx, f.scope, p.Key, p.RequestID); err != nil {
			t.Fatal(err)
		}
		results = append(results, result)
	}
	expected, err := learning.MemoryCombinationHash([]experience.Reference{{VersionID: memory.ID, BodyHash: memory.BodyHash, DeliveryRevision: memory.DeliveryRevision}})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 4 || withMemory.Load() != 2 {
		t.Fatal("memory was not injected exclusively into candidate fullFlow", calls.Load(), withMemory.Load())
	}
	if results[0].CompiledHash != results[1].CompiledHash || results[0].RoleHash != results[1].RoleHash || results[0].MemoryHash != evaluation.EmptyMemoryHash || results[1].MemoryHash != expected {
		t.Fatal("actual memory benefit identities", results)
	}
}
