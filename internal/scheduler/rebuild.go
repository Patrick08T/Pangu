package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
)

type rebuildLookup func(context.Context, string) (*RunDetails, error)

type rebuildState struct {
	previous *submission.Record
	intent   *approval.RebuildIntent
}

// RebuildInvalidated is an explicit, authenticated command, never a maintenance action.
func (b Backend) RebuildInvalidated(ctx context.Context, runID, intentID string) (*submission.Record, error) {
	return rebuildInvalidated(ctx, b.Namespace(), runID, intentID, b.GetRun, b.sendSubmission)
}

func rebuildInvalidated(ctx context.Context, namespace, runID, intentID string, lookup rebuildLookup, send func(context.Context, *submission.Record) (string, error)) (*submission.Record, error) {
	state, err := loadRebuild(ctx, namespace, runID, intentID)
	if err != nil {
		return nil, err
	}
	if err = execution.RequestCancel(ctx, namespace, state.previous.RunID); err != nil {
		return nil, err
	}
	if err = checkRebuildStopped(ctx, state.previous.RunID, lookup); err != nil {
		return nil, err
	}
	if err = checkRebuildSource(ctx, namespace, state.previous); err != nil {
		return nil, err
	}
	if err = approval.ReadyRebuild(ctx, namespace, state.intent.RunID, state.intent.NodeID, state.intent.ToGeneration); err != nil {
		return nil, err
	}
	item, err := prepareRebuild(ctx, namespace, state, lookup)
	if err != nil {
		return nil, err
	}
	return submission.Dispatch(ctx, namespace, item.ID, func(ctx context.Context, item *submission.Record) (string, error) {
		if err := guardRebuildSubmission(ctx, namespace, item, lookup); err != nil {
			return "", err
		}
		return send(ctx, item)
	})
}
