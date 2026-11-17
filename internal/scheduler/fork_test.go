package scheduler

import (
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func TestForkPreviewRejectsUnfinishedAndUnsuccessfulCheckpoint(t *testing.T) {
	d := workflow.Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Input: `{"enabled":true}`, InputSchema: &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"enabled": {Type: "boolean"}}, Required: []string{"enabled"}}, Nodes: []workflow.Node{{ID: "code", Executor: "codex", Instruction: "实现"}}}
	run := &RunDetails{RunSummary: RunSummary{ProjectID: d.ProjectID}, Definition: &d}
	input := workflow.ForkInput{CheckpointNode: "code", Workflow: d}
	if _, err := PreviewFork(run, input); !errors.Is(err, ErrActive) {
		t.Fatalf("运行未停仍允许预览 %v", err)
	}
	run.StopConfirmed = true
	if _, err := PreviewFork(run, input); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("未成功节点可复用 %v", err)
	}
	run.Nodes = []NodeRun{{RunSummary: RunSummary{Status: "succeeded"}, NodeID: "code", Executor: "codex"}}
	if ids, err := PreviewFork(run, input); err != nil || len(ids) != 1 {
		t.Fatalf("合法预览 %+v %v", ids, err)
	}
	input.Workflow.Input = `{}`
	if _, err := PreviewFork(run, input); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("输入必填缺失 %v", err)
	}
}

func TestForkReusesSuccessfulConditionalPrefixButRejectsSkippedOrLatestFailed(t *testing.T) {
	d := workflow.Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Input: `{"enabled":true}`, InputSchema: &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"enabled": {Type: "boolean"}}, Required: []string{"enabled"}}, Nodes: []workflow.Node{
		{ID: "plan", Executor: "codex", Instruction: "规划"},
		{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}, When: &workflow.Condition{Field: workflow.FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`true`)}},
		{ID: "review", Executor: "codex", Instruction: "复核", ReadOnly: true, DependsOn: []string{"build"}},
	}}
	run := &RunDetails{RunSummary: RunSummary{ProjectID: d.ProjectID}, Definition: &d, StopConfirmed: true, Nodes: []NodeRun{
		{RunSummary: RunSummary{Status: "succeeded"}, NodeID: "plan", Executor: "codex"},
		{RunSummary: RunSummary{Status: "succeeded"}, NodeID: "build", Executor: "codex"},
		{RunSummary: RunSummary{Status: "succeeded"}, NodeID: "review", Executor: "codex"},
	}}
	input := workflow.ForkInput{CheckpointNode: "review", Workflow: d}
	if ids, err := PreviewFork(run, input); err != nil || len(ids) != 3 {
		t.Fatalf("已成功条件节点不能复用: %v %v", ids, err)
	}
	for _, status := range []string{"skipped", "failed"} {
		run.Nodes[1].Status = status
		if _, err := PreviewFork(run, input); err == nil {
			t.Fatalf("错误复用 %s 前缀", status)
		}
	}
}
