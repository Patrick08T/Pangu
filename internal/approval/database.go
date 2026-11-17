package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrNotFound = errors.New("审批不存在")
var ErrConflict = errors.New("审批状态或内容已变化")
var ErrExpired = errors.New("审批已过期")
var ErrInvalid = errors.New("审批参数无效")

type Gate struct {
	ApprovalGeneration int                     `json:"approval_generation"`
	InvalidationReason string                  `json:"invalidation_reason,omitempty"`
	InvalidatedAt      *time.Time              `json:"invalidated_at,omitempty"`
	Repair             *Repair                 `json:"repair,omitempty"`
	Purpose            string                  `json:"purpose,omitempty"`
	Execution          *catalog.Execution      `json:"execution,omitempty"`
	InheritedFrom      string                  `json:"inherited_from,omitempty"`
	Round              int                     `json:"round"`
	PreviousID         string                  `json:"previous_id,omitempty"`
	Review             *workflow.Review        `json:"review,omitempty"`
	Items              []workflow.ReviewAnswer `json:"items"`
	ID                 string                  `json:"id"`
	ProjectID          string                  `json:"project_id"`
	RunID              string                  `json:"run_id"`
	NodeID             string                  `json:"node_id"`
	OwnerID            string                  `json:"owner_id"`
	SubjectHash        string                  `json:"subject_hash"`
	Subject            json.RawMessage         `json:"subject"`
	HeadCommit         string                  `json:"head_commit"`
	Status             string                  `json:"status"`
	ActorID            string                  `json:"actor_id"`
	Reason             string                  `json:"reason"`
	CreatedAt          time.Time               `json:"created_at"`
	DecidedAt          *time.Time              `json:"decided_at"`
	ConsumedAt         *time.Time              `json:"consumed_at"`
}

const columns = `a.id::text,a.project_id::text,a.run_id::text,a.node_id,coalesce(s.created_by::text,''),a.subject_hash,a.subject,a.head_commit,
 a.status,coalesce(a.actor_id::text,''),a.reason,a.created_at,a.decided_at,a.consumed_at,a.round,coalesce(a.previous_id::text,''),a.items,a.approval_generation,a.invalidation_reason,a.invalidated_at`

func scan(row pgx.Row) (*Gate, error) {
	out := &Gate{}
	err := row.Scan(&out.ID, &out.ProjectID, &out.RunID, &out.NodeID, &out.OwnerID, &out.SubjectHash, &out.Subject, &out.HeadCommit, &out.Status, &out.ActorID, &out.Reason, &out.CreatedAt, &out.DecidedAt, &out.ConsumedAt, &out.Round, &out.PreviousID, &out.Items, &out.ApprovalGeneration, &out.InvalidationReason, &out.InvalidatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil && diagnostic.Sensitive(out.Subject) {
		return nil, ErrInvalid
	}
	if err == nil && len(out.Subject) > 0 {
		var subject struct {
			Input         workflow.NodeInput `json:"node_input"`
			Review        *workflow.Review   `json:"review"`
			Execution     *catalog.Execution `json:"execution"`
			InheritedFrom string             `json:"inherited_from"`
		}
		if err = json.Unmarshal(out.Subject, &subject); err == nil {
			out.Review = subject.Review
			out.Execution, out.InheritedFrom = subject.Execution, subject.InheritedFrom
			out.Purpose = subject.Input.ReviewPurpose
		}
	}
	return out, err
}

func inputHash(input workflow.NodeInput) (string, error) {
	input.ApprovalID, input.RequestID = "", ""
	if input.ApprovalGeneration == 1 {
		input.ApprovalGeneration = 0 // Keep historical generation-one hashes stable.
	}
	body, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func Request(ctx context.Context, namespace, request, run string, input workflow.NodeInput) (*Gate, error) {
	if input.Node.Approval == nil || input.Node.Validate() != nil {
		return nil, ErrInvalid
	}
	input, err := reportInput(ctx, run, input)
	if err != nil {
		return nil, err
	}
	if input.ApprovalRound == 0 {
		input.ApprovalRound = 1
	}
	review, err := inputReview(input)
	if err != nil {
		return nil, err
	}
	hash, err := inputHash(input)
	if err != nil {
		return nil, err
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
	input.ApprovalGeneration = normalizedGeneration(input.ApprovalGeneration)
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return nil, err
	}
	if err = checkGeneration(ctx, tx, namespace, run, input.Node.ID, input.ApprovalGeneration); err != nil {
		return nil, err
	}
	if err = checkRebuildScope(ctx, tx, namespace, request, run, input); err != nil {
		return nil, err
	}
	var existingHash string
	err = tx.QueryRow(ctx, "SELECT input_hash FROM pangu.approvals WHERE namespace=$1 AND request_id=$2 AND run_id=$3 AND node_id=$4 AND round=$5 AND approval_generation=$6", namespace, request, run, input.Node.ID, input.ApprovalRound, input.ApprovalGeneration).Scan(&existingHash)
	if err == nil && existingHash != hash {
		return nil, ErrConflict
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if err = create(ctx, tx, namespace, request, run, hash, input, review); err != nil {
			return nil, err
		}
	}
	out, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.run_id=$2 AND a.node_id=$3 AND a.input_hash=$4 AND a.round=$5 AND a.approval_generation=$6", namespace, run, input.Node.ID, hash, input.ApprovalRound, input.ApprovalGeneration))
	if errors.Is(err, ErrNotFound) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	if err = checkGateGeneration(ctx, tx, namespace, out); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func create(ctx context.Context, tx pgx.Tx, namespace, request, run, hash string, input workflow.NodeInput, review *workflow.Review) error {
	previous, err := previousRound(ctx, tx, namespace, run, input, review)
	if err != nil {
		return err
	}
	var snapshot json.RawMessage
	var head string
	err = tx.QueryRow(ctx, `SELECT s.snapshot,coalesce(w.head_commit,'') FROM pangu.submissions s LEFT JOIN pangu.workspace_runs w ON w.run_id=$3
 WHERE s.namespace=$1 AND s.id=$2 AND s.project_id=$4 AND s.purged_at IS NULL`, namespace, request, run, input.ProjectID).Scan(&snapshot, &head)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	subject, inherited, err := approvalSubject(ctx, tx, namespace, run, snapshot, input, head, review)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(subject)
	if diagnostic.Sensitive(subject) {
		return ErrInvalid
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.approvals(id,namespace,project_id,request_id,run_id,node_id,input_hash,subject_hash,subject,head_commit,round,previous_id,approval_generation)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,'')::uuid,$13) ON CONFLICT(run_id,node_id,approval_generation,round) DO NOTHING`, uuid.NewString(), namespace, input.ProjectID, request, run, input.Node.ID, hash, hex.EncodeToString(digest[:]), subject, head, input.ApprovalRound, previous, input.ApprovalGeneration)
	if err != nil || inherited == nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.approvals SET status='approved',actor_id=$5,items=$6,decided_at=$7 WHERE run_id=$1 AND node_id=$2 AND round=$3 AND input_hash=$4 AND status='waiting' AND approval_generation=$8`, run, input.Node.ID, input.ApprovalRound, hash, inherited.ActorID, inherited.Items, inherited.DecidedAt, input.ApprovalGeneration)
	return err
}

func Get(ctx context.Context, namespace, id string) (*Gate, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	gate, err := scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.id=$2", namespace, id))
	if err != nil {
		return nil, err
	}
	gate.Repair, _, err = readRepair(ctx, conn, id)
	return gate, err
}

func List(ctx context.Context, namespace, run string) ([]Gate, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.run_id=$2 ORDER BY a.created_at,a.id", namespace, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Gate{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func Waiting(ctx context.Context, run string) (bool, error) {
	state, err := RunState(ctx, run)
	return state == "waiting_approval", err
}

func RunState(ctx context.Context, run string) (string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	var state string
	err = conn.QueryRow(ctx, `SELECT coalesce((SELECT CASE status WHEN 'waiting' THEN 'waiting_approval' WHEN 'changes_requested' THEN 'revising_plan' ELSE '' END
 FROM pangu.approvals a WHERE run_id=$1 AND subject IS NOT NULL AND invalidated_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM pangu.approvals newer WHERE newer.run_id=a.run_id AND newer.node_id=a.node_id AND (newer.approval_generation,newer.round)>(a.approval_generation,a.round))
 ORDER BY CASE status WHEN 'waiting' THEN 0 WHEN 'changes_requested' THEN 1 ELSE 2 END LIMIT 1),'')`, run).Scan(&state)
	return state, err
}
