package approval

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
	"unicode/utf8"
)

type Decision struct {
	RepairHash  string                  `json:"repair_hash,omitempty"`
	SubjectHash string                  `json:"subject_hash"`
	Decision    string                  `json:"decision"`
	Reason      string                  `json:"reason"`
	Items       []workflow.ReviewAnswer `json:"items"`
}

func decideReview(ctx context.Context, actor identity.Actor, id string, input Decision) (*Gate, error) {
	if input.Items == nil {
		input.Items = []workflow.ReviewAnswer{}
	}
	digest, err := hex.DecodeString(input.SubjectHash)
	raw, marshalErr := json.Marshal(input)
	if err != nil || marshalErr != nil || len(raw) > 1024*1024 || len(digest) != 32 || utf8.RuneCountInString(input.Reason) > 4000 || diagnostic.Sensitive(raw) {
		return nil, ErrInvalid
	}
	if actor.TokenID != "" {
		return nil, identity.ErrForbidden
	}
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
	if err = identity.Lock(ctx, tx, actor.Namespace); err != nil {
		return nil, err
	}
	gate, err := currentGate(ctx, tx, actor.Namespace, id)
	if err != nil {
		return nil, err
	}
	if err = identity.Authorize(ctx, actor, gate.ProjectID, "manage_run", gate.OwnerID); err != nil {
		return nil, err
	}
	if err = lockActiveRun(ctx, tx, gate.RunID); err != nil {
		return nil, err
	}
	if gate.Status == "expired" || gate.Subject == nil {
		return nil, ErrExpired
	}
	if gate.SubjectHash != input.SubjectHash || gate.Status == "consumed" {
		return nil, ErrConflict
	}
	if err = validateGateDecision(gate, input); err != nil {
		return nil, err
	}
	if input.Decision == "approved" && gate.Execution != nil && len(gate.Execution.Changes) > 0 {
		if err = identity.Authorize(ctx, actor, "", "manage_catalog", ""); err != nil {
			return nil, err
		}
	}
	if gate.Status == input.Decision && gate.ActorID == actor.ID && gate.Reason == input.Reason && sameAnswers(gate.Items, input.Items) {
		return gate, nil
	}
	if gate.Status != "waiting" && !(gate.Status == "approved" && input.Decision == "revoked") {
		return nil, ErrConflict
	}
	if input.Decision == "approved" || input.Decision == "changes_requested" {
		if err = checkHead(ctx, tx, gate); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.approvals SET status=$2,actor_id=$3,reason=$4,items=$5,decided_at=now(),signalled_at=NULL WHERE id=$1", id, input.Decision, actor.ID, input.Reason, input.Items)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return Get(ctx, actor.Namespace, id)
}

func checkHead(ctx context.Context, tx pgx.Tx, gate *Gate) error {
	var head string
	var expired bool
	err := tx.QueryRow(ctx, "SELECT coalesce((SELECT head_commit FROM pangu.workspace_runs WHERE run_id=$1),''),coalesce((SELECT expired_at IS NOT NULL FROM pangu.workspace_runs WHERE run_id=$1),false)", gate.RunID).Scan(&head, &expired)
	if err != nil {
		return err
	}
	if expired {
		return ErrExpired
	}
	if head != gate.HeadCommit {
		return ErrConflict
	}
	return nil
}

func Consume(ctx context.Context, run string, input workflow.NodeInput, id string) error {
	if input.ApprovalRound == 0 {
		input.ApprovalRound = 1
	}
	input, err := reportInput(ctx, run, input)
	if err != nil {
		return err
	}
	hash, err := inputHash(input)
	if err != nil {
		return err
	}
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
	var namespace string
	err = tx.QueryRow(ctx, "SELECT namespace FROM pangu.approvals WHERE id=$1 AND run_id=$2 AND node_id=$3 AND input_hash=$4", id, run, input.Node.ID, hash).Scan(&namespace)
	if err != nil {
		return ErrConflict
	}
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return err
	}
	gate, err := currentGate(ctx, tx, namespace, id)
	if err != nil {
		return err
	}
	if gate.Status == "consumed" {
		return replayConsumption(ctx, tx, gate)
	}
	if gate.Status == "expired" || gate.Subject == nil {
		return ErrExpired
	}
	if gate.Status != "approved" && !(gate.Status == "changes_requested" && gate.Purpose == "evidence") {
		return ErrConflict
	}
	if err = lockActiveRun(ctx, tx, gate.RunID); err != nil {
		return err
	}
	actor := identity.Actor{ID: gate.ActorID, Namespace: namespace}
	if err = identity.Authorize(ctx, actor, gate.ProjectID, "manage_run", gate.OwnerID); err != nil {
		return err
	}
	allowed, err := identity.ScheduleOwner(ctx, tx, gate.OwnerID, gate.ProjectID)
	if err != nil {
		return err
	}
	if !allowed {
		return identity.ErrForbidden
	}
	if err = checkHead(ctx, tx, gate); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.approvals SET status='consumed',consumed_at=now() WHERE id=$1", id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replayConsumption(ctx context.Context, tx pgx.Tx, gate *Gate) error {
	if err := lockActiveRun(ctx, tx, gate.RunID); err != nil {
		return err
	}
	// 补证只授权只读复验；持久重放时 HEAD 可能已有后续检查点。
	if gate.Purpose == "evidence" {
		return nil
	}
	return checkHead(ctx, tx, gate)
}
