package approval

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func generationInput(t *testing.T, input workflow.NodeInput, generation int) workflow.NodeInput {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err = json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	values["approval_generation"] = generation
	raw, err = json.Marshal(values)
	if err != nil || json.Unmarshal(raw, &input) != nil {
		t.Fatal("cannot encode generation input", err)
	}
	return input
}

func generationFixture(t *testing.T, status string) (identity.Actor, string, string, workflow.NodeInput, *Gate) {
	t.Helper()
	actor, project, run, request := approvalFixture(t)
	input := structuredInput(project, "原范围")
	gate, err := Request(context.Background(), actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	if status == "waiting" {
		return actor, request, run, input, gate
	}
	decision := Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: acceptedItems()}
	if status == "changes_requested" {
		decision.Decision, decision.Reason, decision.Items = status, "人工收窄范围", nil
	}
	if _, err = Decide(context.Background(), actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
	if status == "consumed" {
		if err = Consume(context.Background(), run, input, gate.ID); err != nil {
			t.Fatal(err)
		}
	}
	return actor, request, run, input, gate
}

func TestApprovalInvalidationRejectsLateDecisionAndConsumption(t *testing.T) {
	for _, status := range []string{"waiting", "approved", "consumed", "changes_requested"} {
		t.Run(status, func(t *testing.T) {
			actor, request, run, input, gate := generationFixture(t, status)
			ctx := context.Background()
			intent, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked")
			if err != nil || intent.FromGeneration != 1 || intent.ToGeneration != 2 {
				t.Fatalf("invalidation failed: %+v %v", intent, err)
			}
			if intent.RequiresStop != (status == "consumed") {
				t.Fatalf("wrong stop barrier: %+v", intent)
			}
			old, err := Get(ctx, actor.Namespace, gate.ID)
			if err != nil || old.InvalidationReason != "dependency revoked" || old.InvalidatedAt == nil {
				t.Fatalf("invalidation missing: %+v %v", old, err)
			}
			want := "invalidated"
			if status == "consumed" || status == "changes_requested" {
				want = status
			}
			if old.Status != want || string(old.Subject) != string(gate.Subject) {
				t.Fatal("historical decision or input was overwritten", old.Status)
			}
			decision := Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: acceptedItems()}
			if _, err = Decide(ctx, actor, gate.ID, decision); !errors.Is(err, ErrConflict) {
				t.Fatal("accepted stale decision", err)
			}
			if err = Consume(ctx, run, input, gate.ID); !errors.Is(err, ErrConflict) {
				t.Fatal("accepted stale consumption", err)
			}
			if _, err = Request(ctx, actor.Namespace, request, run, input); !errors.Is(err, ErrConflict) {
				t.Fatal("restored invalidated input", err)
			}
		})
	}
}

func TestApprovalConcurrentInvalidationCreatesOneRecoverableIntent(t *testing.T) {
	actor, _, run, input, _ := generationFixture(t, "approved")
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan *RebuildIntent, 4)
	errors := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			intent, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked")
			results <- intent
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	pending, err := RebuildIntents(ctx, actor.Namespace, run)
	if err != nil || len(pending) != 1 {
		t.Fatalf("recovery intents: %+v %v", pending, err)
	}
	for intent := range results {
		if intent.ID != pending[0].ID || intent.ToGeneration != 2 {
			t.Fatalf("duplicate rebuild: %+v", intent)
		}
	}
}

func TestApprovalGenerationCannotBeSkippedOrChangedInPlace(t *testing.T) {
	actor, request, run, input, _ := generationFixture(t, "waiting")
	ctx := context.Background()
	for _, generation := range []int{-1, 2, 4} {
		_, err := Request(ctx, actor.Namespace, request, run, generationInput(t, input, generation))
		if err == nil {
			t.Fatal("unplanned generation accepted", generation)
		}
	}
	changed := input
	changed.Input = "unapproved replacement"
	if _, err := Request(ctx, actor.Namespace, request, run, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("input mutated in place", err)
	}
}

func TestApprovalGenerationRebuildRequiresExplicitReadiness(t *testing.T) {
	actor, request, run, input, _ := generationFixture(t, "approved")
	ctx := context.Background()
	intent, err := Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "dependency revoked")
	if err != nil {
		t.Fatal(err)
	}
	next := generationInput(t, structuredInput(input.ProjectID, "重建后的范围"), 2)
	if _, err = Request(ctx, actor.Namespace, request, run, next); !errors.Is(err, ErrConflict) {
		t.Fatal("new gate bypassed rebuild readiness", err)
	}
	if err = ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err = ReadyRebuild(ctx, actor.Namespace, run, input.Node.ID, 2); err != nil {
		t.Fatal("readiness retry failed", err)
	}
	gate, err := Request(ctx, actor.Namespace, request, run, next)
	if err != nil || gate.ApprovalGeneration != 2 || gate.Round != 1 || gate.Status != "waiting" {
		t.Fatalf("new generation not isolated: %+v %v", gate, err)
	}
	pending, err := RebuildIntents(ctx, actor.Namespace, run)
	if err != nil || len(pending) != 1 || pending[0].ID != intent.ID || pending[0].ReadyAt == nil {
		t.Fatalf("readiness not durable: %+v %v", pending, err)
	}
	if err = CheckGeneration(ctx, actor.Namespace, run, input.Node.ID, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("dispatch check accepted old generation", err)
	}
	if err = CheckGeneration(ctx, actor.Namespace, run, input.Node.ID, 2); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalGenerationMigrationKeepsLegacyHash(t *testing.T) {
	actor, request, run, input, gate := generationFixture(t, "waiting")
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var generation int
	if err = conn.QueryRow(ctx, "SELECT approval_generation FROM pangu.approvals WHERE id=$1", gate.ID).Scan(&generation); err != nil || generation != 1 {
		t.Fatal("initial database generation", generation, err)
	}
	repeated, err := Request(ctx, actor.Namespace, request, run, generationInput(t, input, 1))
	if err != nil || repeated.ID != gate.ID || repeated.SubjectHash != gate.SubjectHash {
		t.Fatalf("explicit generation broke legacy replay: %+v %v", repeated, err)
	}
}
