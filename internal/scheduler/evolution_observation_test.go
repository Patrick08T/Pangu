//go:build integration

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func evolutionBusinessFixture(t *testing.T, f evolutionFixture, version, input, decision string, execute ...bool) string {
	t.Helper()
	ctx := context.Background()
	d, err := catalog.Compile(ctx, f.scope.Namespace, version, f.scope.ProjectID, input)
	if err != nil {
		t.Fatal(err)
	}
	request, err := submission.Prepare(ctx, f.scope.Namespace, "submit", uuid.NewString(), submission.Snapshot{Purpose: "business", Workflow: *d, CreatedBy: f.scope.ActorID})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, f.scope.Namespace, request.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'same-source','same-source','test-worker','isolated-test',true)`, run, f.scope.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(execute) > 0 && execute[0] {
		evolutionBusinessExecute(t, f, run)
	} else {
		summary := map[string]any{"project_id": f.scope.ProjectID, "status": "succeeded", "stop_confirmed": true}
		_, err = conn.Exec(ctx, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,$3,$4)`, run, f.scope.Namespace, time.Now().Add(time.Second), evolutionJSON(summary))
		if err != nil {
			t.Fatal(err)
		}
	}
	// Frozen server-side entrance provenance, not a client-supplied score.
	_, err = conn.Exec(ctx, `UPDATE pangu.submissions SET snapshot=snapshot||jsonb_build_object('entry_key',$2::text),payload_hash=payload_hash WHERE id=$1`, request.ID, "workbench:"+f.request.FlowObjectID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluation.SaveFeedback(ctx, f.scope.Namespace, f.scope.ProjectID, run, identity.Actor{ID: f.scope.ActorID, Namespace: f.scope.Namespace}, evaluation.FeedbackInput{SubjectHash: evolutionTextHash(run), Decision: decision, Reason: "test human acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestEvolutionBusinessComparisonReadsFeedbackAndRejectsForeignPurpose(t *testing.T) {
	f := evolutionBusinessMemoryFixture(t)
	ctx := context.Background()
	old := f.request.Payload.FlowVersion
	next := evolutionNextFlowFixture(t, f, "")
	baseline := evolutionBusinessFixture(t, f, old, "same paired input", "needs_changes", true)
	candidate := evolutionBusinessFixture(t, f, next.ID, "same paired input", "accepted", true)
	err := evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO pangu.evolution_bindings(namespace,project_id,entry_key,adopted_version,stable_version) VALUES($1,$2,$3,$4,$5)`, f.scope.Namespace, f.scope.ProjectID, "workbench:"+f.request.FlowObjectID, next.ID, old)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	var evidence evolution.BusinessEvidence
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		var e error
		evidence, e = (EvolutionEvidence{}).ReadObservationTx(ctx, tx, f.scope, candidate)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.BaselineRunID != baseline || evidence.CandidateScore != 1 || evidence.BaselineScore != 0 || evidence.SourceHash == "" {
		t.Fatalf("not derived from paired retained evidence: %+v", evidence)
	}
	var original []byte
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		if e := tx.QueryRow(ctx, `SELECT snapshot FROM pangu.submissions WHERE run_id=$1`, candidate).Scan(&original); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{workflow,nodes,0,instruction}','"unpublished override"') WHERE run_id=$1`, candidate)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, e := (EvolutionEvidence{}).ReadObservationTx(ctx, tx, f.scope, candidate)
		return e
	})
	if err == nil {
		t.Fatal("unpublished instruction override masqueraded as adopted version")
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE pangu.submissions SET snapshot=$2 WHERE run_id=$1`, candidate, original)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{purpose}','"evaluation"') WHERE run_id=$1`, candidate)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	err = evolutionTransaction(ctx, func(tx pgx.Tx) error {
		_, e := (EvolutionEvidence{}).ReadObservationTx(ctx, tx, f.scope, candidate)
		return e
	})
	if err == nil {
		t.Fatal("evaluation output was accepted as business observation")
	}
}
