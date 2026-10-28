package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

var ErrActive = errors.New("运行仍有未确认停止的节点，不能整次重新运行")

func (b Backend) Rerun(ctx context.Context, id, key string) (*submission.Record, error) {
	run, err := b.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if !run.StopConfirmed {
		return nil, ErrActive
	}
	id = run.ID
	namespace := b.client.NewWorkflow(WorkflowName).GetName()
	previous, err := submission.ForRun(ctx, namespace, id)
	if err != nil {
		return nil, err
	}
	snapshot := previous.Snapshot
	snapshot.SourceRunID = id
	item, err := submission.FindRerun(ctx, namespace, previous.ProjectID, key, id)
	if err == nil {
		return b.dispatch(ctx, item), nil
	}
	if !errors.Is(err, submission.ErrNotFound) {
		return nil, err
	}
	expired, err := workspace.Expired(ctx, id)
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, workspace.ErrExpired
	}
	check, err := b.preflight(ctx, preflightInput{Workflow: snapshot.Workflow, SourceRunID: id})
	if err != nil {
		return nil, err
	}
	if !check.Ready {
		return nil, ErrNotReady
	}
	item, err = submission.Prepare(ctx, namespace, "rerun", key, snapshot)
	if err != nil {
		return nil, err
	}
	return b.dispatch(ctx, item), nil
}
