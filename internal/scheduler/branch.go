package scheduler

import (
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func Outputs(run *RunDetails) map[string]workflow.NodeOutput {
	out := map[string]workflow.NodeOutput{}
	if run.Definition == nil {
		return out
	}
	for _, node := range run.Definition.Nodes {
		for _, result := range run.Nodes {
			if result.NodeID != node.ID {
				continue
			}
			if result.Status == "skipped" {
				out[node.ID] = workflow.NodeOutput{Skipped: true}
				continue
			}
			value := workflow.NodeOutput{Text: result.Output, RunID: result.ID}
			if result.Status == "succeeded" && workflow.ValidateOutput(node, &value) == nil {
				out[node.ID] = value
			}
		}
	}
	return out
}

func enrichBranches(run *RunDetails, record *submission.Record) {
	if record != nil && record.Snapshot.Fork != nil {
		run.Fork = record.Snapshot.Fork
		for id, out := range run.Fork.Reused {
			node := derivedNode(run, id, "codex", "succeeded", out.Text)
			node.ReusedFrom = run.SourceRunID
			run.Nodes = append(run.Nodes, node)
		}
	}
	if run.Definition == nil {
		return
	}
	previews, err := workflow.PreviewInputs(*run.Definition, Outputs(run))
	if err != nil {
		return
	}
	for _, item := range previews {
		if item.State == "skipped" {
			for _, node := range run.Definition.Nodes {
				if node.ID == item.NodeID {
					run.Nodes = append(run.Nodes, derivedNode(run, node.ID, node.Executor, "skipped", "条件未命中或直接上游已跳过"))
				}
			}
		}
	}
}

func derivedNode(run *RunDetails, id, executor, status, output string) NodeRun {
	virtual := uuid.NewSHA1(uuid.MustParse(run.ID), []byte(id)).String()
	return NodeRun{RunSummary: RunSummary{ID: virtual, Status: status, CreatedAt: run.CreatedAt}, TaskID: virtual, NodeID: id, Executor: executor, Output: output}
}
