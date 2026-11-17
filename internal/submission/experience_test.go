package submission

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestSubmissionFreezesExplicitEmptyExperienceAtAcceptance(t *testing.T) {
	project := submissionProject(t)
	ctx := context.Background()
	snap := Snapshot{Workflow: workflow.Definition{ProjectID: project, Input: "original"}}
	first, err := Prepare(ctx, "submission-test", "submit", "memory-empty", snap)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var before []byte
	err = conn.QueryRow(ctx, `SELECT snapshot FROM pangu.experience_watermarks WHERE namespace='submission-test' AND project_id=$1 AND run_id=$2 AND generation=1`, project, first.ID).Scan(&before)
	if err != nil {
		t.Fatalf("accepted request has no frozen experience watermark: %v", err)
	}
	var mark struct {
		Hash     string
		Versions []json.RawMessage
	}
	if err = json.Unmarshal(before, &mark); err != nil || len(mark.Hash) != 64 || len(mark.Versions) != 0 {
		t.Fatalf("disabled project did not freeze explicit empty selection: %s %v", before, err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES('submission-test',$1,1,'test-owner','{"read_enabled":true}')`, project)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Prepare(ctx, "submission-test", "submit", "memory-empty", snap)
	if err != nil || again.ID != first.ID {
		t.Fatalf("unchanged accepted request lost idempotency after policy change: %+v %v", again, err)
	}
	var after []byte
	err = conn.QueryRow(ctx, `SELECT snapshot FROM pangu.experience_watermarks WHERE namespace='submission-test' AND project_id=$1 AND run_id=$2 AND generation=1`, project, first.ID).Scan(&after)
	if err != nil || string(after) != string(before) {
		t.Fatalf("retry changed frozen candidates: %s %v", after, err)
	}
}

func TestAnonymousSubmissionCannotAcceptEnabledMemoryPolicy(t *testing.T) {
	project := submissionProject(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES('submission-test',$1,1,'test-owner','{"read_enabled":true}')`, project)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Prepare(ctx, "submission-test", "submit", "memory-anonymous", Snapshot{Workflow: workflow.Definition{ProjectID: project}})
	if err == nil {
		t.Fatal("anonymous submission accepted an enabled experience policy")
	}
	var requests int
	err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.submissions WHERE namespace='submission-test' AND project_id=$1`, project).Scan(&requests)
	if err != nil || requests != 0 {
		t.Fatalf("failed memory acceptance left an orphan dispatchable submission: %d %v", requests, err)
	}
}

func TestSubmissionAndExperienceWatermarkShareTransaction(t *testing.T) {
	project := submissionProject(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	request, err := PrepareTx(ctx, tx, "submission-test", "submit", "memory-rollback", Snapshot{Workflow: workflow.Definition{ProjectID: project}})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM pangu.experience_watermarks WHERE namespace='submission-test' AND project_id=$1 AND run_id=$2`, project, request.ID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("transaction lacks request watermark: %d %v", count, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	err = conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM pangu.experience_watermarks WHERE project_id=$1)+(SELECT count(*) FROM pangu.submissions WHERE project_id=$1)`, project).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("rollback retained accepted input or watermark: %d %v", count, err)
	}
}
