package submission

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func TestRerunsKeepBusinessTaskIdentity(t *testing.T) {
	project := submissionProject(t)
	ctx := context.Background()
	first, err := Prepare(ctx, "submission-test", "submit", "first", Snapshot{Workflow: workflow.Definition{ProjectID: project}})
	if err != nil {
		t.Fatal(err)
	}
	source := uuid.NewString()
	if _, err = Dispatch(ctx, "submission-test", first.ID, func(context.Context, *Record) (string, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	second, err := Prepare(ctx, "submission-test", "rerun", "retry", Snapshot{Workflow: workflow.Definition{ProjectID: project}, SourceRunID: source})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var one, two string
	if err = conn.QueryRow(ctx, "SELECT task_id::text FROM pangu.submissions WHERE id=$1", first.ID).Scan(&one); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(ctx, "SELECT task_id::text FROM pangu.submissions WHERE id=$1", second.ID).Scan(&two); err != nil {
		t.Fatal(err)
	}
	if one != first.ID || two != one {
		t.Fatalf("重跑放大业务任务数: %s %s", one, two)
	}
	other := submissionProject(t)
	if _, err = Prepare(ctx, "submission-test", "rerun", "foreign", Snapshot{Workflow: workflow.Definition{ProjectID: other}, SourceRunID: source}); err == nil {
		t.Fatal("跨项目继承任务身份")
	}
}
