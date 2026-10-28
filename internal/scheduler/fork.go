package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func (b Backend) Fork(ctx context.Context, id, key string, input workflow.ForkInput) (*submission.Record, error) {
	if !submission.ValidKey(key) {
		return nil, catalog.ErrInvalid
	}
	source, err := uuid.Parse(id)
	if err != nil {
		return nil, catalog.ErrInvalid
	}
	id = source.String()
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	requestHash := fmt.Sprintf("%x", sha256.Sum256(raw))
	item, err := submission.FindFork(ctx, b.Namespace(), input.Workflow.ProjectID, key, id, requestHash)
	if err == nil {
		return b.dispatch(ctx, item), nil
	}
	if !errors.Is(err, submission.ErrNotFound) {
		return nil, err
	}
	run, err := b.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	previous, err := submission.ForRun(ctx, b.Namespace(), id)
	if err != nil {
		return nil, err
	}
	snapshot, err := forkSnapshot(ctx, run, previous, input)
	if err != nil {
		return nil, err
	}
	snapshot.Fork.RequestHash = requestHash
	if err = catalog.CheckDefinition(ctx, b.Namespace(), input.Workflow); err != nil {
		return nil, err
	}
	check, err := b.preflight(ctx, preflightInput{Workflow: input.Workflow, SourceRunID: id})
	if err != nil {
		return nil, err
	}
	if !check.Ready {
		return nil, ErrNotReady
	}
	item, err = submission.Prepare(ctx, b.Namespace(), "fork", key, *snapshot)
	if err != nil {
		return nil, err
	}
	return b.dispatch(ctx, item), nil
}

func forkSnapshot(ctx context.Context, run *RunDetails, previous *submission.Record, input workflow.ForkInput) (*submission.Snapshot, error) {
	ids, err := PreviewFork(run, input)
	if err != nil {
		return nil, err
	}
	source, err := workspace.RerunSource(ctx, run.ID, previous.ProjectID)
	if err != nil {
		return nil, err
	}
	release, err := workspace.Lease(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	defer release()
	fork := &workflow.Fork{CheckpointNode: input.CheckpointNode, OriginCommit: source.BaseCommit, Reused: map[string]workflow.NodeOutput{}, Checkpoints: map[string]string{}}
	if previous.Snapshot.Fork != nil {
		fork.OriginCommit = previous.Snapshot.Fork.OriginCommit
	}
	fork.Commit = fork.OriginCommit
	// 目标必须真实成功，即使输入变化导致不再复用它。
	if _, _, err = successfulCheckpoint(ctx, run, input.CheckpointNode); err != nil {
		return nil, err
	}
	for _, id := range ids {
		commit, out, err := successfulCheckpoint(ctx, run, id)
		if err != nil {
			return nil, err
		}
		fork.Commit = commit
		fork.Checkpoints[id] = commit
		fork.Reused[id] = out
	}
	return &submission.Snapshot{Workflow: input.Workflow, SourceRunID: run.ID, Fork: fork}, nil
}

func successfulCheckpoint(ctx context.Context, run *RunDetails, node string) (string, workflow.NodeOutput, error) {
	for _, item := range run.Nodes {
		if item.NodeID == node && item.Executor == "codex" && item.Status == "succeeded" {
			return workspace.Checkpoint(ctx, run.ID, item.ID)
		}
	}
	return "", workflow.NodeOutput{}, errors.Join(catalog.ErrInvalid, errors.New("选中节点没有成功的 Codex 文件检查点"))
}

func PreviewFork(run *RunDetails, input workflow.ForkInput) ([]string, error) {
	if !run.StopConfirmed {
		return nil, ErrActive
	}
	if run.LogsExpired || run.Definition == nil {
		return nil, workspace.ErrExpired
	}
	if input.Workflow.ProjectID != run.ProjectID {
		return nil, catalog.ErrInvalid
	}
	if err := input.Workflow.ValidateInput(); err != nil {
		return nil, errors.Join(catalog.ErrInvalid, err)
	}
	for _, node := range run.Nodes {
		if node.NodeID == input.CheckpointNode && node.Executor == "codex" && node.Status == "succeeded" {
			ids, err := workflow.Prefix(input.Workflow, *run.Definition, input.CheckpointNode)
			if err != nil {
				return nil, errors.Join(catalog.ErrInvalid, err)
			}
			return ids, nil
		}
	}
	return nil, errors.Join(catalog.ErrInvalid, errors.New("选中节点没有成功的 Codex 文件检查点"))
}
