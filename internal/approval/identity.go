package approval

import (
	"context"
	"encoding/hex"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
	"strings"
)

type Decision struct {
	SubjectHash string `json:"subject_hash"`
	Decision    string `json:"decision"`
	Reason      string `json:"reason"`
}

func Decide(ctx context.Context, actor identity.Actor, id string, input Decision) (*Gate, error) {
	digest, err := hex.DecodeString(input.SubjectHash)
	if err != nil || len(digest) != 32 || (input.Decision != "approved" && input.Decision != "rejected" && input.Decision != "revoked") || strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 2048 || diagnostic.Sensitive([]byte(input.Reason)) {
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
	gate, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.id=$2 FOR UPDATE OF a", actor.Namespace, id))
	if err != nil {
		return nil, err
	}
	if err = identity.Authorize(ctx, actor, gate.ProjectID, "manage_run", gate.OwnerID); err != nil {
		return nil, err
	}
	if gate.Status == "expired" || gate.Subject == nil {
		return nil, ErrExpired
	}
	if gate.SubjectHash != input.SubjectHash || gate.Status == "consumed" {
		return nil, ErrConflict
	}
	if gate.Status == input.Decision && gate.ActorID == actor.ID && gate.Reason == input.Reason {
		return gate, nil
	}
	if gate.Status != "waiting" && !(gate.Status == "approved" && input.Decision == "revoked") {
		return nil, ErrConflict
	}
	if input.Decision == "approved" {
		if err = checkHead(ctx, tx, gate); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.approvals SET status=$2,actor_id=$3,reason=$4,decided_at=now(),signalled_at=NULL WHERE id=$1", id, input.Decision, actor.ID, input.Reason)
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
	gate, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.id=$1 FOR UPDATE OF a", id))
	if err != nil {
		return err
	}
	if gate.Status == "consumed" {
		return checkHead(ctx, tx, gate)
	}
	if gate.Status == "expired" || gate.Subject == nil {
		return ErrExpired
	}
	if gate.Status != "approved" {
		return ErrConflict
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
