package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
)

func prepareRebuild(ctx context.Context, namespace string, state *rebuildState, lookup rebuildLookup) (*submission.Record, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return nil, err
	}
	current, err := loadRebuild(ctx, namespace, state.previous.RunID, state.intent.ID)
	if err != nil {
		return nil, err
	}
	if err = checkRebuildStopped(ctx, current.previous.RunID, lookup); err != nil {
		return nil, err
	}
	if err = checkRebuildSource(ctx, namespace, current.previous); err != nil {
		return nil, err
	}
	if err = authorizeRebuild(ctx, namespace, current.previous); err != nil {
		return nil, err
	}
	if err = approval.CheckGeneration(ctx, namespace, current.intent.RunID, current.intent.NodeID, current.intent.ToGeneration); err != nil {
		return nil, err
	}
	linked, err := approval.RebuildSuccessor(ctx, namespace, current.intent.ID)
	if err != nil {
		return nil, err
	}
	if linked != "" {
		return submission.Get(ctx, namespace, linked)
	}
	snapshot := submission.Snapshot{Purpose: current.previous.Snapshot.Purpose, Workflow: current.previous.Snapshot.Workflow, CreatedBy: current.previous.CreatedBy, SourceRunID: current.previous.RunID}
	item, err := submission.PrepareTx(ctx, tx, namespace, "rebuild", "rebuild:"+current.intent.ID, snapshot)
	if err != nil {
		return nil, err
	}
	if err = approval.LinkRebuildTx(ctx, tx, namespace, current.intent.ID, item.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return item, nil
}
