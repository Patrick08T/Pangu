package approval

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

// RebuildSuccessor returns the durable successor request ID, or empty when unlinked.
func RebuildSuccessor(ctx context.Context, namespace, intentID string) (string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	return rebuildSuccessor(ctx, conn, namespace, intentID)
}

func rebuildSuccessor(ctx context.Context, q generationReader, namespace, intentID string) (string, error) {
	var request string
	err := q.QueryRow(ctx, `SELECT s.new_request_id::text FROM pangu.approval_rebuild_successors s
 JOIN pangu.approval_rebuild_intents i ON i.id=s.intent_id WHERE i.namespace=$1 AND i.id=$2`, namespace, intentID).Scan(&request)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return request, err
}

// LinkRebuild binds a prepared successor; callers creating it should prefer LinkRebuildTx.
func LinkRebuild(ctx context.Context, namespace, intentID, newRequest string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = LinkRebuildTx(ctx, tx, namespace, intentID, newRequest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LinkRebuildTx must share PrepareTx's transaction to prevent unlinked dispatch.
// Acquire identity.Lock before preparing the submission to preserve lock order.
func LinkRebuildTx(ctx context.Context, tx pgx.Tx, namespace, intentID, newRequest string) error {
	if err := identity.Lock(ctx, tx, namespace); err != nil {
		return err
	}
	existing, err := rebuildSuccessor(ctx, tx, namespace, intentID)
	if err != nil {
		return err
	}
	if existing != "" {
		if existing != newRequest {
			return ErrConflict
		}
		return nil
	}
	intent, err := readRebuild(tx.QueryRow(ctx, "SELECT "+rebuildColumns+" FROM pangu.approval_rebuild_intents WHERE namespace=$1 AND id=$2 FOR UPDATE", namespace, intentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if intent.ReadyAt == nil {
		return ErrConflict
	}
	if err = checkGeneration(ctx, tx, namespace, intent.RunID, intent.NodeID, intent.ToGeneration); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "submission:"+newRequest); err != nil {
		return err
	}
	if err = validateRebuildSuccessor(ctx, tx, namespace, newRequest, intent); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, "INSERT INTO pangu.approval_rebuild_successors(intent_id,new_request_id) VALUES($1,$2) ON CONFLICT DO NOTHING", intent.ID, newRequest)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func validateRebuildSuccessor(ctx context.Context, tx pgx.Tx, namespace, newRequest string, intent *RebuildIntent) error {
	var oldRaw, newRaw []byte
	err := tx.QueryRow(ctx, `SELECT old.snapshot,fresh.snapshot FROM pangu.approval_rebuild_intents i
 JOIN pangu.submissions old ON old.id=i.request_id JOIN pangu.submissions fresh ON fresh.id=$3
 WHERE i.namespace=$1 AND i.id=$2 AND fresh.namespace=$1 AND old.project_id=fresh.project_id
 AND old.created_by IS NOT DISTINCT FROM fresh.created_by AND old.purged_at IS NULL AND fresh.purged_at IS NULL
 AND fresh.run_id IS NULL AND fresh.last_checked_at='-infinity'::timestamptz
 AND NOT EXISTS(SELECT 1 FROM pangu.approvals WHERE request_id=fresh.id)
 AND NOT EXISTS(SELECT 1 FROM pangu.approvals WHERE run_id=i.run_id AND node_id=i.node_id AND approval_generation>=i.to_generation)
 FOR UPDATE OF fresh`, namespace, intent.ID, newRequest).Scan(&oldRaw, &newRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	var old, fresh submission.Snapshot
	if err = json.Unmarshal(oldRaw, &old); err != nil {
		return err
	}
	if err = json.Unmarshal(newRaw, &fresh); err != nil {
		return err
	}
	if !sameSuccessorScope(old, fresh, intent.RunID) {
		return ErrConflict
	}
	return nil
}

func sameSuccessorScope(old, fresh submission.Snapshot, run string) bool {
	a, aErr := json.Marshal(old.Workflow)
	b, bErr := json.Marshal(fresh.Workflow)
	return aErr == nil && bErr == nil && string(a) == string(b) && old.CreatedBy == fresh.CreatedBy && fresh.SourceRunID == run && fresh.ApprovalOrigin == "" && fresh.Fork == nil
}

func checkLinkedRebuildScope(ctx context.Context, tx pgx.Tx, namespace, request string, input workflow.NodeInput) error {
	var prior json.RawMessage
	err := tx.QueryRow(ctx, `SELECT a.subject FROM pangu.approval_rebuild_successors link
 JOIN pangu.approval_rebuild_intents i ON i.id=link.intent_id
 JOIN pangu.approvals a ON a.run_id=i.run_id AND a.node_id=i.node_id AND a.approval_generation=i.from_generation
 WHERE i.namespace=$1 AND link.new_request_id=$2 AND i.node_id=$3 ORDER BY a.round DESC LIMIT 1`, namespace, request, input.Node.ID).Scan(&prior)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return compareRebuildSubject(prior, input)
}

func compareRebuildSubject(prior json.RawMessage, input workflow.NodeInput) error {
	if len(prior) == 0 {
		return ErrConflict
	}
	var subject struct {
		Input workflow.NodeInput `json:"node_input"`
	}
	if err := json.Unmarshal(prior, &subject); err != nil {
		return err
	}
	if !sameRebuildScope(subject.Input, input) {
		return ErrConflict
	}
	return nil
}
