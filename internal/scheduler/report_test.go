package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func planReport(body string) workflow.NodeOutput {
	report := workflow.Report{ID: uuid.NewString(), Title: "完整任务计划", Markdown: body}
	return workflow.NodeOutput{Text: `{"ready":true}`, Reports: []workflow.Report{report}, ReportIDs: []string{report.ID}, ReportSHA256: workflow.ReportHash(body)}
}

func TestReadonlyReportCheckpointAndForkKeepWholeBody(t *testing.T) {
	run, project := revisionRun(t)
	ctx := context.Background()
	body := `{"ready":true,"tasks":["` + strings.Repeat("任务与验收", 6000) + `"]}`
	item := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), Attempt: 1}
	in := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "plan", Executor: "codex", Instruction: "规划", ReadOnly: true}}
	out, err := executeNode(ctx, item, in, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) { return planReport(body), nil })
	if err != nil {
		t.Fatal(err)
	}
	commit, restored, err := workspace.Checkpoint(ctx, run, item.NodeRunID)
	if err != nil || restored.ReportSHA256 != out.ReportSHA256 {
		t.Fatalf("恢复丢失报告: %v", err)
	}
	data, _ := json.Marshal(restored)
	if len(data) > 16384 {
		t.Fatal("检查点信封仍含报告正文")
	}
	read := func(runID string, parent workflow.NodeOutput) {
		t.Helper()
		input := workflow.NodeInput{RunID: runID, ProjectID: project, Node: workflow.Node{DependsOn: []string{"plan"}}, Parents: map[string]workflow.NodeOutput{"plan": parent}}
		parents, err := research.ParentContext(ctx, input)
		if err != nil || parents["plan"].Text != body {
			t.Fatalf("交接没有保留完整报告: %v", err)
		}
	}
	read(run, restored)
	next := uuid.NewString()
	if err = workspace.EnsureFork(ctx, next, project, "revision", run, commit); err != nil {
		t.Fatal(err)
	}
	fork := &workflow.Fork{Checkpoints: map[string]string{"plan": commit}, Reused: map[string]workflow.NodeOutput{"plan": restored}}
	values, err := workspace.ImportCheckpoints(ctx, next, fork)
	if err != nil {
		t.Fatal(err)
	}
	read(next, values["plan"])
	repeated, err := workspace.ImportCheckpoints(ctx, next, fork)
	if err != nil || repeated["plan"].ReportIDs[0] != values["plan"].ReportIDs[0] {
		t.Fatalf("返工恢复不幂等: %v", err)
	}
}

func TestRejectedReadOnlyAttemptDoesNotPublishReport(t *testing.T) {
	run, project := revisionRun(t)
	ctx := context.Background()
	item := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), Attempt: 1}
	in := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "plan", Executor: "codex", Instruction: "规划", ReadOnly: true}}
	_, err := executeNode(ctx, item, in, func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		return planReport(`{"ready":true}`), os.WriteFile(filepath.Join(input.Workspace, "越权写入.txt"), []byte("不能发布"), 0600)
	})
	if err == nil {
		t.Fatal("只读节点写入成功")
	}
	reports, err := research.Reports(ctx, run)
	if err != nil || len(reports) != 0 {
		t.Fatalf("失败节点泄漏已发布报告: %v", err)
	}
}

func TestRevisionRejectsUnregisteredHead(t *testing.T) {
	run, project := revisionRun(t)
	ctx := context.Background()
	item := execution.Attempt{RunID: run, NodeRunID: uuid.NewString(), Attempt: 1}
	in := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "plan", Executor: "codex", Instruction: "规划", ReadOnly: true}}
	_, err := executeNode(ctx, item, in, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "完成"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "DELETE FROM pangu.workspace_checkpoints WHERE run_id=$1", run); err != nil {
		t.Fatal(err)
	}
	item.NodeRunID = uuid.NewString()
	_, err = executeNode(ctx, item, in, func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error) {
		t.Fatal("未登记检查点进入模型执行")
		return workflow.NodeOutput{}, nil
	})
	if err == nil {
		t.Fatal("未拒绝陌生检查点")
	}
}
