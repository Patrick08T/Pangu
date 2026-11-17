package scheduler

import (
	"context"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/catalog"

	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
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
	snapshot.ApprovalOrigin = ""
	snapshot.SourceRunID = id
	if snapshot.Fork != nil {
		snapshot.Fork = &workflow.Fork{OriginCommit: snapshot.Fork.OriginCommit, Commit: snapshot.Fork.OriginCommit}
	}
	item, err := submission.FindRerun(ctx, namespace, previous.ProjectID, key, id)
	if err == nil {
		return b.dispatch(ctx, item), nil
	}
	if !errors.Is(err, submission.ErrNotFound) {
		return nil, err
	}
	if err := validateRerun(snapshot.Workflow); err != nil {
		return nil, fmt.Errorf("%w：原运行快照不符合当前契约；整次重跑固定原版本，请从工作流页面选择新版发起运行：%w", catalog.ErrInvalid, err)
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

func validateRerun(d workflow.Definition) error {
	if err := d.ValidateHandoffs(); err != nil {
		return err
	}
	if err := d.ValidateInput(); err != nil {
		return err
	}
	_, err := d.Layers()
	return err
}
