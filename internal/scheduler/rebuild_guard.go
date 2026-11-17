package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/jackc/pgx/v5"
)

// Keep this guard in sendSubmission too: pending reconciliation is not user authority.
func (b Backend) checkRebuildSubmission(ctx context.Context, item *submission.Record) error {
	return guardRebuildSubmission(ctx, b.Namespace(), item, b.GetRun)
}

func guardRebuildSubmission(ctx context.Context, namespace string, item *submission.Record, lookup rebuildLookup) error {
	if item.Operation != "rebuild" {
		return nil
	}
	intentID, err := rebuildIntentForRequest(ctx, namespace, item.ID)
	if err != nil {
		return err
	}
	state, err := loadRebuild(ctx, namespace, item.Snapshot.SourceRunID, intentID)
	if err != nil {
		return err
	}
	if item.CreatedBy != state.previous.CreatedBy || item.Snapshot.Fork != nil || item.Snapshot.ApprovalOrigin != "" {
		return approval.ErrConflict
	}
	if err = checkRebuildStopped(ctx, state.previous.RunID, lookup); err != nil {
		return err
	}
	if err = checkRebuildSource(ctx, namespace, state.previous); err != nil {
		return err
	}
	if err = approval.CheckGeneration(ctx, namespace, state.intent.RunID, state.intent.NodeID, state.intent.ToGeneration); err != nil {
		return err
	}
	return authorizeRebuild(ctx, namespace, state.previous)
}

func rebuildIntentForRequest(ctx context.Context, namespace, request string) (string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	var intent string
	err = conn.QueryRow(ctx, `SELECT i.id::text FROM pangu.approval_rebuild_successors s
 JOIN pangu.approval_rebuild_intents i ON i.id=s.intent_id WHERE i.namespace=$1 AND s.new_request_id=$2`, namespace, request).Scan(&intent)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", approval.ErrConflict
	}
	return intent, err
}

func checkRebuildStopped(ctx context.Context, runID string, lookup rebuildLookup) error {
	cancelled, err := execution.CancelRequested(ctx, runID)
	if err != nil {
		return err
	}
	if !cancelled {
		return approval.ErrConflict
	}
	run, err := lookup(ctx, runID)
	if err != nil {
		return err
	}
	if run == nil || run.ID != runID || !run.StopConfirmed || !terminal(run.Status) {
		return approval.ErrConflict
	}
	for _, node := range run.Nodes {
		if !terminal(node.Status) {
			return approval.ErrConflict
		}
	}
	active, err := execution.Active(ctx, runID)
	if err == nil && active != 0 {
		return approval.ErrConflict
	}
	return err
}

func checkRebuildSource(ctx context.Context, namespace string, previous *submission.Record) error {
	// Share the worker's validator; a fork or its rerun may have a polluted BaseCommit.
	_, err := workspace.RebuildSource(ctx, previous.RunID, previous.ProjectID, namespace)
	return err
}
