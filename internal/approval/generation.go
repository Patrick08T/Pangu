package approval

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RebuildIntent records invalidation, not compensation or proof of stopped execution.
type RebuildIntent struct {
	ID             string     `json:"id"`
	RunID          string     `json:"run_id"`
	NodeID         string     `json:"node_id"`
	FromGeneration int        `json:"from_generation"`
	ToGeneration   int        `json:"to_generation"`
	Reason         string     `json:"reason"`
	RequiresStop   bool       `json:"requires_stop"`
	CreatedAt      time.Time  `json:"created_at"`
	ReadyAt        *time.Time `json:"ready_at,omitempty"`
}

func sameRebuildScope(a, b workflow.NodeInput) bool {
	scope := func(in workflow.NodeInput) []byte {
		// Compare the whole Node, including approval and future policy fields.
		raw, _ := json.Marshal(struct {
			ProjectID, Input, ReviewPurpose string
			Node                            workflow.Node
		}{in.ProjectID, in.Input, in.ReviewPurpose, in.Node})
		return raw
	}
	return string(scope(a)) == string(scope(b))
}

func checkRebuildScope(ctx context.Context, tx pgx.Tx, namespace, request, run string, input workflow.NodeInput) error {
	if input.ApprovalGeneration == 1 {
		return checkLinkedRebuildScope(ctx, tx, namespace, request, input)
	}
	var prior json.RawMessage
	err := tx.QueryRow(ctx, `SELECT a.subject FROM pangu.approval_rebuild_intents i JOIN pangu.approvals a
 ON a.run_id=i.run_id AND a.node_id=i.node_id AND a.approval_generation=i.from_generation
 WHERE i.namespace=$1 AND i.request_id=$2 AND i.run_id=$3 AND i.node_id=$4 AND i.to_generation=$5
 AND NOT EXISTS(SELECT 1 FROM pangu.approval_rebuild_successors WHERE intent_id=i.id)
 ORDER BY a.round DESC LIMIT 1`, namespace, request, run, input.Node.ID, input.ApprovalGeneration).Scan(&prior)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && len(prior) == 0) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	return compareRebuildSubject(prior, input)
}

const rebuildColumns = `id::text,run_id::text,node_id,from_generation,to_generation,reason,requires_stop,created_at,ready_at`

type generationReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func normalizedGeneration(generation int) int {
	if generation == 0 {
		return 1
	}
	return generation
}

func readRebuild(row pgx.Row) (*RebuildIntent, error) {
	out := &RebuildIntent{}
	err := row.Scan(&out.ID, &out.RunID, &out.NodeID, &out.FromGeneration, &out.ToGeneration, &out.Reason, &out.RequiresStop, &out.CreatedAt, &out.ReadyAt)
	return out, err
}

func currentGeneration(ctx context.Context, q generationReader, namespace, run, node string) (int, bool, error) {
	var current int
	var ready bool
	err := q.QueryRow(ctx, `WITH current AS (
 SELECT greatest(coalesce((SELECT max(approval_generation) FROM pangu.approvals WHERE namespace=$1 AND run_id=$2 AND node_id=$3),1),
 coalesce((SELECT max(to_generation) FROM pangu.approval_rebuild_intents WHERE namespace=$1 AND run_id=$2 AND node_id=$3),1)) AS generation)
 SELECT generation,NOT EXISTS(SELECT 1 FROM pangu.approval_rebuild_intents WHERE namespace=$1 AND run_id=$2 AND node_id=$3 AND to_generation=generation AND ready_at IS NULL) FROM current`, namespace, run, node).Scan(&current, &ready)
	return current, ready, err
}

func checkGeneration(ctx context.Context, q generationReader, namespace, run, node string, generation int) error {
	generation = normalizedGeneration(generation)
	if generation < 1 {
		return ErrInvalid
	}
	current, ready, err := currentGeneration(ctx, q, namespace, run, node)
	if err != nil {
		return err
	}
	if current != generation || !ready {
		return ErrConflict
	}
	return nil
}

// CheckGeneration revalidates a frozen generation immediately before dispatch.
func CheckGeneration(ctx context.Context, namespace, run, node string, generation int) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if err = checkGeneration(ctx, conn, namespace, run, node, generation); err != nil {
		return err
	}
	gate, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.run_id=$2 AND a.node_id=$3 AND a.approval_generation=$4 ORDER BY a.round DESC LIMIT 1", namespace, run, node, normalizedGeneration(generation)))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return checkGateGeneration(ctx, conn, namespace, gate)
}

func checkGateGeneration(ctx context.Context, q generationReader, namespace string, gate *Gate) error {
	seen := map[string]bool{}
	for {
		if gate.InvalidatedAt != nil || seen[gate.ID] {
			return ErrConflict
		}
		seen[gate.ID] = true
		if err := checkGeneration(ctx, q, namespace, gate.RunID, gate.NodeID, gate.ApprovalGeneration); err != nil {
			return err
		}
		if gate.InheritedFrom == "" {
			return nil
		}
		var err error
		gate, err = scan(q.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.id=$2", namespace, gate.InheritedFrom))
		if err != nil {
			return err
		}
		if gate.Status != "consumed" || gate.Subject == nil {
			return ErrConflict
		}
	}
}

func currentGate(ctx context.Context, tx pgx.Tx, namespace, id string) (*Gate, error) {
	gate, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.id=$2 FOR UPDATE OF a", namespace, id))
	if err != nil {
		return nil, err
	}
	if err = checkGateGeneration(ctx, tx, namespace, gate); err != nil {
		return nil, err
	}
	return gate, nil
}

// Invalidate compares the old generation and durably records exactly one rebuild intent.
func Invalidate(ctx context.Context, namespace, run, node string, expectedGeneration int, reason string) (*RebuildIntent, error) {
	expectedGeneration = normalizedGeneration(expectedGeneration)
	raw, _ := json.Marshal(reason)
	if namespace == "" || node == "" || expectedGeneration < 1 || strings.TrimSpace(reason) == "" || utf8.RuneCountInString(reason) > 4000 || diagnostic.Sensitive(raw) {
		return nil, ErrInvalid
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
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return nil, err
	}
	existing, err := readRebuild(tx.QueryRow(ctx, "SELECT "+rebuildColumns+" FROM pangu.approval_rebuild_intents WHERE namespace=$1 AND run_id=$2 AND node_id=$3 AND from_generation=$4", namespace, run, node, expectedGeneration))
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	out, err := invalidateGeneration(ctx, tx, namespace, run, node, expectedGeneration, reason)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func invalidateGeneration(ctx context.Context, tx pgx.Tx, namespace, run, node string, generation int, reason string) (*RebuildIntent, error) {
	if err := checkGeneration(ctx, tx, namespace, run, node, generation); err != nil {
		return nil, err
	}
	var request string
	var stop, expired bool
	err := tx.QueryRow(ctx, `SELECT request_id::text,subject IS NULL OR status='expired',EXISTS(SELECT 1 FROM pangu.approvals WHERE namespace=$1 AND run_id=$2 AND node_id=$3 AND approval_generation=$4 AND consumed_at IS NOT NULL)
 FROM pangu.approvals WHERE namespace=$1 AND run_id=$2 AND node_id=$3 AND approval_generation=$4 ORDER BY round DESC LIMIT 1`, namespace, run, node, generation).Scan(&request, &expired, &stop)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, ErrExpired
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.approvals SET
 status=CASE WHEN status IN ('waiting','approved') AND consumed_at IS NULL THEN 'invalidated' ELSE status END,
 invalidation_reason=$5,invalidated_at=now(),signalled_at=NULL
 WHERE namespace=$1 AND run_id=$2 AND node_id=$3 AND approval_generation=$4 AND invalidated_at IS NULL AND subject IS NOT NULL`, namespace, run, node, generation, reason)
	if err != nil {
		return nil, err
	}
	return readRebuild(tx.QueryRow(ctx, `INSERT INTO pangu.approval_rebuild_intents(id,namespace,request_id,run_id,node_id,from_generation,to_generation,reason,requires_stop)
 VALUES($1,$2,$3,$4,$5,$6,$6+1,$7,$8) RETURNING `+rebuildColumns, uuid.NewString(), namespace, request, run, node, generation, reason, stop))
}

// ReadyRebuild is an internal scheduler acknowledgement of safe stop and clean rebuild.
// It must not be called until StopConfirmed (when required) and dependency checks pass.
func ReadyRebuild(ctx context.Context, namespace, run, node string, generation int) error {
	if generation < 2 {
		return ErrInvalid
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
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return err
	}
	current, _, err := currentGeneration(ctx, tx, namespace, run, node)
	if err != nil {
		return err
	}
	if current != generation {
		return ErrConflict
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.approval_rebuild_intents SET ready_at=coalesce(ready_at,now()) WHERE namespace=$1 AND run_id=$2 AND node_id=$3 AND to_generation=$4`, namespace, run, node, generation)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

// RebuildIntents returns durable history, including ready intents, for idempotent recovery.
func RebuildIntents(ctx context.Context, namespace, run string) ([]RebuildIntent, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+rebuildColumns+" FROM pangu.approval_rebuild_intents WHERE namespace=$1 AND run_id=$2 ORDER BY created_at,id", namespace, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RebuildIntent{}
	for rows.Next() {
		intent, err := readRebuild(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *intent)
	}
	return out, rows.Err()
}
