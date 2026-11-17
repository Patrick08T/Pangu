//go:build integration

package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func evolutionBusinessMemoryFixture(t *testing.T) evolutionFixture {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { evolutionResponse(w, true) }))
	t.Cleanup(server.Close)
	f := newEvolutionFixture(t, server.URL)
	entryExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy)
 SELECT namespace,project_id,2,actor_id,policy||'{"revision":2,"read_enabled":true,"delivery":{"mode":"serving","task_scope":["submit","entry"],"traffic_basis_points":10000}}'::jsonb
 FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=1`, f.scope.Namespace, f.scope.ProjectID)
	f.request.PolicyRevision = 2
	f.service.Memory = learning.NewStore()
	return f
}

func evolutionBusinessMemoryVersion(t *testing.T, f evolutionFixture) experience.Version {
	t.Helper()
	source := actualBenefitSource(t, f)
	content := experience.Content{Kind: "practice", Observation: "JSON fields", Practice: "Check required keys", Conditions: "JSON task", Counterexamples: "Not prose", Applicability: experience.Applicability{Roles: []string{f.request.AgentObjectID}, Keywords: []string{"JSON"}}}
	v, err := f.service.Memory.Create(context.Background(), experience.Scope{Namespace: f.scope.Namespace, ProjectID: f.scope.ProjectID, ActorID: f.scope.ActorID}, experience.Version{Revision: 1, ProjectID: f.scope.ProjectID, ExpiresAt: time.Now().Add(30 * time.Minute), Content: content, Sources: []experience.Source{source}})
	if err != nil {
		t.Fatal(err)
	}
	// Eligibility only. Selection, injected payloads, execution and artifacts below are real.
	entryExec(t, `UPDATE pangu.experience_states SET validity='active' WHERE version_id=$1`, v.ID)
	entryExec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,2,'serving','{"mode":"serving","task_scope":["submit","entry"],"traffic_basis_points":10000}',$2)`, v.ID, f.scope.ActorID)
	return v
}

func evolutionBusinessExecute(t *testing.T, f evolutionFixture, run string) {
	t.Helper()
	ctx := context.Background()
	request, err := submission.ForRun(ctx, f.scope.Namespace, run)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := loadNodeExperience(ctx, f.scope.Namespace, request.ID, run, f.scope.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []NodeRun{}
	_, err = runtime.Run(ctx, request.Snapshot.Workflow, nil, nil, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		outputs := []workflow.NodeOutput{}
		for _, input := range inputs {
			output, node := evolutionBusinessExecuteNode(t, f, run, input)
			outputs, nodes = append(outputs, output), append(nodes, node)
		}
		return outputs, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	summary := map[string]any{"project_id": f.scope.ProjectID, "status": "succeeded", "stop_confirmed": true, "nodes": nodes}
	entryExec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,clock_timestamp(),$3)`, run, f.scope.Namespace, evolutionJSON(summary))
}

func evolutionBusinessExecuteNode(t *testing.T, f evolutionFixture, run string, input workflow.NodeInput) (workflow.NodeOutput, NodeRun) {
	t.Helper()
	ctx := context.Background()
	attempt := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), NodeID: input.Node.ID}
	if err := execution.Begin(ctx, &attempt); err != nil {
		t.Fatal(err)
	}
	output, err := executeExperienced(ctx, f.scope.Namespace, attempt, input, agent.Execute)
	attempt.Status = "succeeded"
	if err != nil {
		attempt.Status = "failed"
	}
	if finishErr := execution.Finish(ctx, attempt); finishErr != nil {
		t.Fatal(finishErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return output, NodeRun{RunSummary: RunSummary{ID: attempt.NodeRunID, Status: "succeeded"}, NodeID: input.Node.ID, Attempt: attempt.Attempt}
}

func evolutionBusinessRead(t *testing.T, f evolutionFixture, run string) (evolutionBusinessRun, error) {
	t.Helper()
	var out evolutionBusinessRun
	err := evolutionTransaction(context.Background(), func(tx pgx.Tx) error {
		var err error
		out, err = evolutionBusinessRunTx(context.Background(), tx, f.scope, run)
		return err
	})
	return out, err
}

func evolutionBusinessEntryRun(t *testing.T, f evolutionFixture, decision string) string {
	t.Helper()
	ctx := identity.WithActor(context.Background(), identity.Actor{ID: f.scope.ActorID, Namespace: f.scope.Namespace})
	request, err := submitEntry(ctx, f.scope.Namespace, f.scope.ProjectID, "main", "JSON business input", uuid.NewString(), entryHooks{entryReady, entryPending})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, f.scope.Namespace, request.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	entryExec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'same-source','same-source','test-worker','isolated-test',true)`, run, f.scope.ProjectID)
	evolutionBusinessExecute(t, f, run)
	_, err = evaluation.SaveFeedback(ctx, f.scope.Namespace, f.scope.ProjectID, run, identity.Current(ctx), evaluation.FeedbackInput{SubjectHash: evolutionTextHash(run), Decision: decision, Reason: "actual human feedback"})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func evolutionBusinessObserve(t *testing.T, f evolutionFixture, run string) (evolution.BusinessEvidence, error) {
	t.Helper()
	var out evolution.BusinessEvidence
	err := evolutionTransaction(context.Background(), func(tx pgx.Tx) error {
		var err error
		out, err = (EvolutionEvidence{}).ReadObservationTx(context.Background(), tx, f.scope, run)
		return err
	})
	return out, err
}
