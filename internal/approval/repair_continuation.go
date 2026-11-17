package approval

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func validateRepairFork(ctx context.Context, tx pgx.Tx, gate *Gate, body *repairBody) error {
	var s struct {
		Snapshot submission.Snapshot `json:"snapshot"`
	}
	if err := json.Unmarshal(gate.Subject, &s); err != nil {
		return err
	}
	if body.Execution.Workflow.ProjectID != gate.ProjectID || body.Execution.BaseVersion != s.Snapshot.Workflow.VersionID {
		return ErrConflict
	}
	fork := body.Fork
	ids, err := workflow.Prefix(body.Execution.Workflow, s.Snapshot.Workflow, fork.CheckpointNode)
	if err != nil || len(ids) == 0 || len(ids) != len(fork.Reused) || ids[len(ids)-1] != fork.CheckpointNode {
		return ErrConflict
	}
	before := map[string]bool{}
	for _, id := range ids {
		before[id] = true
	}
	if err = validateReusedOutputs(ctx, tx, gate.RunID, before, fork); err != nil {
		return err
	}
	if fork.ReportNodes[fork.CheckpointNode] || fork.Commit != fork.Checkpoints[fork.CheckpointNode] {
		return ErrConflict
	}
	var origin string
	if s.Snapshot.Fork != nil {
		origin = s.Snapshot.Fork.OriginCommit
	} else if err = tx.QueryRow(ctx, "SELECT base_commit FROM pangu.workspace_runs WHERE run_id=$1", gate.RunID).Scan(&origin); err != nil {
		return err
	}
	if origin != fork.OriginCommit {
		return ErrConflict
	}
	return nil
}

func ContinueRepair(ctx context.Context, namespace, id string) (*submission.Record, error) {
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
	gate, err := currentGate(ctx, tx, namespace, id)
	if err != nil {
		return nil, err
	}
	var existing string
	err = tx.QueryRow(ctx, "SELECT request_id::text FROM pangu.plan_continuations WHERE approval_id=$1", id).Scan(&existing)
	if err == nil {
		return submission.Get(ctx, namespace, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	r, body, err := readRepair(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if gate.Status != "approved" || gate.Purpose != "evidence" || r == nil || r.Status != "approved" || gate.SubjectHash != body.SubjectHash {
		return nil, ErrConflict
	}
	if err = continuationAuthority(ctx, tx, namespace, gate); err != nil {
		return nil, err
	}
	if err = checkHead(ctx, tx, gate); err != nil {
		return nil, err
	}
	if err = validateRepairFork(ctx, tx, gate, body); err != nil {
		return nil, err
	}
	if err = catalog.CheckRepairDraft(ctx, tx, namespace, body.Execution); err != nil {
		return nil, err
	}
	version, err := catalog.PublishExecutionTx(ctx, tx, namespace, body.Execution)
	if err != nil {
		return nil, err
	}
	d := body.Execution.Workflow
	d.VersionID = version.ID
	item, err := submission.PrepareTx(ctx, tx, namespace, "plan", "plan:"+id, submission.Snapshot{Workflow: d, CreatedBy: gate.OwnerID, SourceRunID: gate.RunID, Fork: body.Fork, ApprovalOrigin: id})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO pangu.plan_continuations(approval_id,request_id,version_id) VALUES($1,$2,$3)", id, item.ID, version.ID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.approvals SET status='consumed',consumed_at=now() WHERE id=$1", id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.evidence_repairs SET status='consumed' WHERE id=$1", r.ID); err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}
