package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestApprovalSuccessorPreservesFrozenAuthority(t *testing.T) {
	old := submission.Snapshot{CreatedBy: "actor", Workflow: workflow.Definition{ProjectID: "project", VersionID: "version", Input: "original input"}}
	fresh := old
	fresh.SourceRunID = "original-run"
	if !sameSuccessorScope(old, fresh, "original-run") {
		t.Fatal("clean same-authority successor rejected")
	}
	mutations := map[string]func(*submission.Snapshot){
		"actor":   func(s *submission.Snapshot) { s.CreatedBy = "other" },
		"project": func(s *submission.Snapshot) { s.Workflow.ProjectID = "other" },
		"version": func(s *submission.Snapshot) { s.Workflow.VersionID = "other" },
		"input":   func(s *submission.Snapshot) { s.Workflow.Input = "broader task" },
		"source":  func(s *submission.Snapshot) { s.SourceRunID = "other" },
		"inherit": func(s *submission.Snapshot) { s.ApprovalOrigin = "old-approval" },
		"reuse":   func(s *submission.Snapshot) { s.Fork = &workflow.Fork{} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := fresh
			mutate(&changed)
			if sameSuccessorScope(old, changed, "original-run") {
				t.Fatal("successor changed frozen authority")
			}
		})
	}
}

func TestApprovalGenerationDoesNotResurrectPurgedSubject(t *testing.T) {
	actor, _, run, input, gate := generationFixture(t, "waiting")
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.approvals SET subject=NULL,status='expired' WHERE id=$1", gate.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = Invalidate(ctx, actor.Namespace, run, input.Node.ID, 1, "must not reintroduce retained payload"); !errors.Is(err, ErrExpired) {
		t.Fatal("purged subject invalidated into a fresh retained payload", err)
	}
	intents, err := RebuildIntents(ctx, actor.Namespace, run)
	if err != nil || len(intents) != 0 {
		t.Fatal("purged gate produced a recovery intent", intents, err)
	}
}
