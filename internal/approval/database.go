package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
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
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	RunID       string          `json:"run_id"`
	NodeID      string          `json:"node_id"`
	OwnerID     string          `json:"owner_id"`
	SubjectHash string          `json:"subject_hash"`
	Subject     json.RawMessage `json:"subject"`
	HeadCommit  string          `json:"head_commit"`
	Status      string          `json:"status"`
	ActorID     string          `json:"actor_id"`
	Reason      string          `json:"reason"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
	DecidedAt   *time.Time      `json:"decided_at"`
	ConsumedAt  *time.Time      `json:"consumed_at"`
}

const columns = `a.id::text,a.project_id::text,a.run_id::text,a.node_id,coalesce(s.created_by::text,''),a.subject_hash,a.subject,a.head_commit,
 CASE WHEN a.status!='consumed' AND a.expires_at<=clock_timestamp() THEN 'expired' ELSE a.status END,coalesce(a.actor_id::text,''),a.reason,a.created_at,a.expires_at,a.decided_at,a.consumed_at`

func scan(row pgx.Row) (*Gate, error) {
	out := &Gate{}
	err := row.Scan(&out.ID, &out.ProjectID, &out.RunID, &out.NodeID, &out.OwnerID, &out.SubjectHash, &out.Subject, &out.HeadCommit, &out.Status, &out.ActorID, &out.Reason, &out.CreatedAt, &out.ExpiresAt, &out.DecidedAt, &out.ConsumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil && diagnostic.Sensitive(out.Subject) {
		return nil, ErrInvalid
	}
	return out, err
}

func inputHash(input workflow.NodeInput) (string, error) {
	input.ApprovalID = ""
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
	var existingHash string
	err = tx.QueryRow(ctx, "SELECT input_hash FROM pangu.approvals WHERE namespace=$1 AND request_id=$2 AND run_id=$3 AND node_id=$4", namespace, request, run, input.Node.ID).Scan(&existingHash)
	if err == nil && existingHash != hash {
		return nil, ErrConflict
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if err = create(ctx, tx, namespace, request, run, hash, input); err != nil {
			return nil, err
		}
	}
	out, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.run_id=$2 AND a.node_id=$3 AND a.input_hash=$4", namespace, run, input.Node.ID, hash))
	if errors.Is(err, ErrNotFound) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func create(ctx context.Context, tx pgx.Tx, namespace, request, run, hash string, input workflow.NodeInput) error {
	var snapshot json.RawMessage
	var head string
	var deadline time.Time
	err := tx.QueryRow(ctx, `SELECT s.snapshot,coalesce(w.head_commit,''),now()+$5*interval '1 second' FROM pangu.submissions s LEFT JOIN pangu.workspace_runs w ON w.run_id=$3
 WHERE s.namespace=$1 AND s.id=$2 AND s.project_id=$4 AND s.purged_at IS NULL`, namespace, request, run, input.ProjectID, input.Node.Approval.TimeoutSeconds).Scan(&snapshot, &head, &deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	subject, err := json.Marshal(struct {
		Snapshot json.RawMessage    `json:"snapshot"`
		Input    workflow.NodeInput `json:"node_input"`
		Head     string             `json:"head_commit"`
	}{snapshot, input, head})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(subject)
	if diagnostic.Sensitive(subject) {
		return ErrInvalid
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.approvals(id,namespace,project_id,request_id,run_id,node_id,input_hash,subject_hash,subject,head_commit,expires_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(run_id,node_id) DO NOTHING`, uuid.NewString(), namespace, input.ProjectID, request, run, input.Node.ID, hash, hex.EncodeToString(digest[:]), subject, head, deadline)
	return err
}

func Get(ctx context.Context, namespace, id string) (*Gate, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.id=$2", namespace, id))
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
