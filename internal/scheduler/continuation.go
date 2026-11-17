package scheduler

import (
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func continuePlan(ctx hatchet.DurableContext, client *hatchet.Client, d workflow.Definition, gate approval.Gate, outputs map[string]workflow.NodeOutput) error {
	namespace := client.NewWorkflow(WorkflowName).GetName()
	previous, err := submission.ForRun(ctx.GetContext(), namespace, ctx.WorkflowRunId())
	if err != nil {
		return err
	}
	source, err := workspace.RerunSource(ctx.GetContext(), gate.RunID, gate.ProjectID)
	if err != nil {
		return err
	}
	fork := &workflow.Fork{OriginCommit: source.BaseCommit, Commit: source.BaseCommit, Reused: map[string]workflow.NodeOutput{}, Checkpoints: map[string]string{}, ReportNodes: map[string]bool{}, ReportHashes: map[string]string{}}
	if previous.Snapshot.Fork != nil {
		fork.OriginCommit = previous.Snapshot.Fork.OriginCommit
	}
	layers, err := d.Layers()
	if err != nil {
		return err
	}
	for _, layer := range layers {
		if layer[0].ID == gate.NodeID {
			break
		}
		for _, n := range layer {
			out, ok := outputs[n.ID]
			if !ok || out.Skipped {
				return approval.ErrConflict
			}
			if n.Executor == "codex" {
				fork.Commit, out, err = workspace.Checkpoint(ctx.GetContext(), gate.RunID, out.RunID)
			} else {
				out, err = research.Checkpoint(ctx.GetContext(), gate.RunID, out.RunID)
				fork.ReportNodes[n.ID] = true
			}
			if err != nil {
				return err
			}
			if fork.ReportNodes[n.ID] {
				fork.ReportHashes[n.ID] = workflow.ReportHash(out.Reports[0].Markdown)
			}
			fork.Reused[n.ID], fork.Checkpoints[n.ID] = out, fork.Commit
			fork.CheckpointNode = n.ID
		}
	}
	// 只读并行节点的提交次序可能不同；续跑从审批时实际 HEAD 开始。
	fork.Commit = gate.HeadCommit
	item, err := approval.Continue(ctx.GetContext(), namespace, gate.ID, fork)
	if err != nil {
		return err
	}
	NewBackend(client).dispatch(ctx.GetContext(), item)
	return workflow.ErrContinued
}
