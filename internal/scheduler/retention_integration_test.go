package scheduler_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func TestHatchetExpiredInputKeepsRerunReceiptButRejectsNewRerun(t *testing.T) {
	_, b, handler := integrationSetup(t)
	project := registerTestProject(t, b)
	ctx := context.Background()
	d := workflow.Definition{ProjectID: project.ID, Input: "到期输入", Nodes: []workflow.Node{{ID: "one", Executor: "eino", Model: "one", Instruction: "临时执行指令"}}}
	original, err := b.Submit(ctx, d, "original")
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, handler, original.RunID)
	rerun, err := b.Rerun(ctx, original.RunID, "rerun-key")
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, handler, rerun.RunID)
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, id := range []string{original.RunID, rerun.RunID} {
		if _, err := conn.Exec(ctx, "UPDATE pangu.workspace_runs SET created_at=now()-interval '31 days' WHERE run_id=$1", id); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, "UPDATE pangu.node_attempts SET finished_at=now()-interval '31 days' WHERE run_id=$1", id); err != nil {
			t.Fatal(err)
		}
		if err := workspace.Expire(ctx, id, time.Now().Add(-31*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	details, err := b.GetRun(ctx, original.RunID)
	if err != nil || !details.LogsExpired || details.Definition != nil || len(details.Attempts) != 0 {
		t.Fatalf("过期运行仍暴露临时输入: %+v %v", details, err)
	}
	receipt, err := b.Rerun(ctx, strings.ToUpper(original.RunID), "rerun-key")
	if err != nil || receipt.RunID != rerun.RunID {
		t.Fatalf("快照清理后原重跑回执丢失: %+v %v", receipt, err)
	}
	if _, err := b.Rerun(ctx, original.RunID, "new-key"); !errors.Is(err, workspace.ErrExpired) {
		t.Fatal("过期基准的新重跑未明确拒绝", err)
	}
}
