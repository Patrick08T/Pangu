package submission

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func TestExpiredInputIsRemovedWithoutBreakingVersionIdempotency(t *testing.T) {
	project := submissionProject(t)
	t.Setenv("PANGU_WORKER_ID", "test")
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	ctx := context.Background()
	snapshot := Snapshot{Workflow: workflow.Definition{ProjectID: project, VersionID: uuid.NewString(), Input: "到期后必须删除的输入", Nodes: []workflow.Node{{ID: "one", Executor: "codex", Instruction: "临时指令"}}}}
	record, err := Prepare(ctx, "submission-test", "submit", "expiry", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	run := uuid.NewString()
	if _, err := conn.Exec(ctx, "UPDATE pangu.submissions SET run_id=$2,created_at=now()-interval '31 days' WHERE id=$1", record.ID, run); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Expire(ctx, run, time.Now().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	stored, err := Get(ctx, "submission-test", record.ID)
	if err != nil || stored.Snapshot.Workflow.Input != "" || len(stored.Snapshot.Workflow.Nodes) != 0 {
		t.Fatalf("30天后请求快照仍含临时输入或指令: %+v %v", stored, err)
	}
	receipt, err := FindVersion(ctx, "submission-test", project, "expiry", snapshot.Workflow.VersionID, snapshot.Workflow.Input)
	if err != nil || receipt.RunID != run {
		t.Fatalf("清理输入破坏了90天版本回执: %+v %v", receipt, err)
	}
	if _, err := FindVersion(ctx, "submission-test", project, "expiry", snapshot.Workflow.VersionID, "改变输入"); !errors.Is(err, ErrConflict) {
		t.Fatal("输入清理后同键可以更换输入", err)
	}
	if receipt, err := Find(ctx, "submission-test", "submit", "expiry", snapshot); err != nil || receipt.RunID != run {
		t.Fatal("原始 JSON 提交的摘要校验被破坏", err)
	}
}

func TestLateReceiptCannotRestoreExpiredPayload(t *testing.T) {
	project := submissionProject(t)
	t.Setenv("PANGU_WORKER_ID", "test")
	root := t.TempDir()
	t.Setenv("PANGU_WORKSPACE_ROOT", root)
	ctx := context.Background()
	record, err := Prepare(ctx, "submission-test", "submit", "late", Snapshot{Workflow: workflow.Definition{ProjectID: project, Input: "临时输入"}})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	run := uuid.NewString()
	if _, err := conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,created_at)
 VALUES($1,$2,'base','head','test',$3,now()-interval '31 days')`, run, project, filepath.Join(root, run)); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Expire(ctx, run, time.Now().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	receipt, err := Dispatch(ctx, "submission-test", record.ID, func(context.Context, *Record) (string, error) { return strings.ToUpper(run), nil })
	if err != nil || receipt.RunID != run {
		t.Fatalf("回执失败: %+v %v", receipt, err)
	}
	saved, err := Get(ctx, "submission-test", record.ID)
	if err != nil || saved.Snapshot.Workflow.Input != "" {
		t.Fatalf("迟到关联恢复了已过期正文: %+v %v", saved, err)
	}
}
