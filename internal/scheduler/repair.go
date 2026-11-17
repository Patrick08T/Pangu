package scheduler

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (b Backend) PrepareRepair(ctx context.Context, id string, input approval.RepairInput) (*approval.Repair, error) {
	gate, err := approval.CheckRepairAuthority(ctx, b.Namespace(), id, input.SubjectHash)
	if err != nil {
		return nil, err
	}
	run, err := b.GetRun(ctx, gate.RunID)
	if err != nil {
		return nil, err
	}
	if run.Status != "waiting_approval" || run.CancelRequested || run.StopConfirmed || run.Definition == nil {
		return nil, approval.ErrConflict
	}
	for _, n := range run.Nodes {
		if n.Status == "running" || n.Status == "queued" {
			return nil, ErrActive
		}
	}
	execution, err := catalog.PrepareRepair(ctx, b.Namespace(), *run.Definition, input.ObjectID, input.Revision)
	if err != nil {
		return nil, err
	}
	previous, err := submission.ForRun(ctx, b.Namespace(), gate.RunID)
	if err != nil {
		return nil, err
	}
	// 待审批且无活动节点时允许冻结成功前缀；保存、批准、消费时仍核对运行和 HEAD。
	copy := *run
	copy.StopConfirmed = true
	snapshot, err := forkSnapshot(ctx, &copy, previous, workflow.ForkInput{CheckpointNode: input.CheckpointNode, Workflow: execution.Workflow})
	if err != nil {
		return nil, err
	}
	return approval.SaveRepair(ctx, b.Namespace(), id, input, execution, snapshot.Fork)
}
