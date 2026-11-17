//go:build integration

package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

// Hatchet transport is replaced; authorization, snapshots and archived metadata are real DB reads.
type retentionHTTPBackend struct {
	authorizedBackend
	runs []string
}

func (b *retentionHTTPBackend) GetRun(ctx context.Context, id string) (*scheduler.RunDetails, error) {
	body, err := submission.History(ctx, b.namespace, id)
	if err != nil {
		return nil, err
	}
	var stored struct {
		scheduler.RunDetails
		InternalPurpose string `json:"internal_purpose"`
	}
	if err = json.Unmarshal(body, &stored); err != nil {
		return nil, err
	}
	stored.Purpose = stored.InternalPurpose
	return &stored.RunDetails, nil
}

func (b *retentionHTTPBackend) ListRuns(ctx context.Context, _ scheduler.RunQuery) (*scheduler.RunList, error) {
	out := &scheduler.RunList{Items: []scheduler.RunSummary{}}
	for _, id := range b.runs {
		item, err := b.GetRun(ctx, id)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item.RunSummary)
	}
	return out, nil
}

type retentionRun struct{ id, gate string }

func seedRetentionRun(t *testing.T, f experienceFixture, purpose string, purged bool, historyPurpose string) retentionRun {
	t.Helper()
	ctx := identity.WithActor(context.Background(), identity.Actor{ID: f.actor, Namespace: f.namespace})
	node := workflow.Node{ID: "build", Executor: "codex", Handoff: testHandoff(), Instruction: "implement", DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}
	definition := workflow.Definition{ProjectID: f.project, Input: "retained-task-input", Nodes: []workflow.Node{{ID: "plan", Executor: "codex", Instruction: "plan", Handoff: testHandoff()}, node}}
	item, err := submission.Prepare(ctx, f.namespace, "retention-test", uuid.NewString(), submission.Snapshot{Purpose: purpose, Workflow: definition})
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	if _, err = submission.Dispatch(ctx, f.namespace, item.ID, func(context.Context, *submission.Record) (string, error) { return id, nil }); err != nil {
		t.Fatal(err)
	}
	input := workflow.NodeInput{ProjectID: f.project, Node: node, Parents: map[string]workflow.NodeOutput{"plan": {Text: `{"review":{"title":"scope","summary":"review","items":[{"id":"scope","category":"scope","title":"document","proposal":"document only","rationale":"verify","evidence_refs":[],"task_ids":[]}],"resolutions":[]}}`}}}
	gate, err := approval.Request(ctx, f.namespace, item.ID, id, input)
	if err != nil {
		t.Fatal(err)
	}
	if purged {
		purgeHistoricalRetentionSnapshot(t, f, item.ID)
	}
	storeRetentionHistory(t, f, id, historyPurpose)
	return retentionRun{id: id, gate: gate.ID}
}

func purgeHistoricalRetentionSnapshot(t *testing.T, f experienceFixture, request string) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	// Reproduce the historical pre-0064 purge projection, not the fixed current purge function.
	_, err = conn.Exec(ctx, `UPDATE pangu.submissions SET snapshot=jsonb_build_object('workflow',jsonb_build_object('project_id',project_id::text,'version_id',$3::text,'budget',jsonb_build_object('max_tokens',1000))) WHERE namespace=$1 AND id=$2`, f.namespace, request, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
}

func storeRetentionHistory(t *testing.T, f experienceFixture, id, purpose string) {
	t.Helper()
	out := struct {
		scheduler.RunDetails
		InternalPurpose string `json:"internal_purpose,omitempty"`
	}{RunDetails: scheduler.RunDetails{RunSummary: scheduler.RunSummary{ID: id, ProjectID: f.project, CreatedBy: f.actor, Status: "succeeded", Title: "restricted-metadata-" + id}, LogsExpired: true, StopConfirmed: true}, InternalPurpose: purpose}
	body, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err = submission.SaveHistory(context.Background(), f.namespace, id, time.Now(), body); err != nil {
		t.Fatal(err)
	}
}
