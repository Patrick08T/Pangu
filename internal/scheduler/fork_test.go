package scheduler

import (
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
