package approval

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"unicode/utf8"
)

type RepairInput struct {
	SubjectHash    string `json:"subject_hash"`
	ObjectID       string `json:"object_id"`
	Revision       int    `json:"revision"`
	CheckpointNode string `json:"checkpoint_node"`
	Summary        string `json:"summary"`
}

type Repair struct {
	ID             string             `json:"id"`
	Hash           string             `json:"hash"`
	Status         string             `json:"status"`
	Summary        string             `json:"summary"`
	CheckpointNode string             `json:"checkpoint_node"`
	ReusedNodes    []string           `json:"reused_nodes"`
	ExecuteNodes   []string           `json:"execute_nodes"`
	Execution      *catalog.Execution `json:"execution"`
}

type repairBody struct {
	SubjectHash string             `json:"subject_hash"`
	Summary     string             `json:"summary"`
	Execution   *catalog.Execution `json:"execution"`
	Fork        *workflow.Fork     `json:"fork"`
}

func readRepair(ctx context.Context, tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (*Repair, *repairBody, error) {
	r := &Repair{}
	var raw []byte
	err := tx.QueryRow(ctx, "SELECT id::text,subject_hash,status,body FROM pangu.evidence_repairs WHERE approval_id=$1 AND status<>'superseded' AND body IS NOT NULL", id).Scan(&r.ID, &r.Hash, &r.Status, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var body repairBody
	if err = json.Unmarshal(raw, &body); err != nil {
		return nil, nil, err
	}
	if body.Execution == nil || body.Fork == nil || diagnostic.Sensitive(raw) {
		return nil, nil, ErrInvalid
	}
	canonical, err := json.Marshal(body)
	if err != nil || workflow.ReportHash(string(canonical)) != r.Hash {
		return nil, nil, ErrConflict
	}
	r.Summary, r.Execution, r.CheckpointNode = body.Summary, body.Execution, body.Fork.CheckpointNode
	r.ReusedNodes, r.ExecuteNodes = []string{}, []string{}
	for _, n := range body.Execution.Workflow.Nodes {
		if _, ok := body.Fork.Reused[n.ID]; ok {
			r.ReusedNodes = append(r.ReusedNodes, n.ID)
		} else {
			r.ExecuteNodes = append(r.ExecuteNodes, n.ID)
		}
	}
	return r, &body, nil
}

func SaveRepair(ctx context.Context, namespace, id string, input RepairInput, execution *catalog.Execution, fork *workflow.Fork) (*Repair, error) {
	actor := identity.Current(ctx)
	if actor.Namespace != namespace {
		return nil, identity.ErrForbidden
	}
	if strings.TrimSpace(input.Summary) == "" || utf8.RuneCountInString(input.Summary) > 4000 || execution == nil || fork == nil {
		return nil, ErrInvalid
	}
	body, err := json.Marshal(repairBody{input.SubjectHash, input.Summary, execution, fork})
	if err != nil || diagnostic.Sensitive(body) {
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
	gate, err := lockRepairGate(ctx, tx, actor, id)
	if err != nil {
		return nil, err
	}
	if gate.Purpose != "evidence" || gate.Status != "waiting" || gate.SubjectHash != input.SubjectHash {
		return nil, ErrConflict
	}
	if err = catalog.CheckRepairDraft(ctx, tx, actor.Namespace, execution); err != nil {
		return nil, err
	}
	if err = validateRepairFork(ctx, tx, gate, &repairBody{Execution: execution, Fork: fork}); err != nil {
		return nil, err
	}
	hash := workflow.ReportHash(string(body))
	current, _, err := readRepair(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if current != nil && current.Hash == hash {
		return current, nil
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.evidence_repairs SET status='superseded' WHERE approval_id=$1 AND status='prepared'", id); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO pangu.evidence_repairs(id,approval_id,subject_hash,body,status,created_by) VALUES($1,$2,$3,$4,'prepared',$5)", uuid.NewString(), id, hash, body, actor.ID)
	if err != nil {
		return nil, err
	}
	r, _, err := readRepair(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return r, tx.Commit(ctx)
}

func Decide(ctx context.Context, actor identity.Actor, id string, input Decision) (*Gate, error) {
	if input.RepairHash != "" {
		return DecideRepair(ctx, actor, id, input)
	}
	return decideReview(ctx, actor, id, input)
}

func CheckRepairAuthority(ctx context.Context, namespace, id, hash string) (*Gate, error) {
	actor := identity.Current(ctx)
	if actor.Namespace != namespace || actor.TokenID != "" {
		return nil, identity.ErrForbidden
	}
	gate, err := Get(ctx, namespace, id)
	if err != nil {
		return nil, err
	}
	if err = identity.Authorize(ctx, actor, gate.ProjectID, "manage_run", gate.OwnerID); err != nil {
		return nil, err
	}
	if err = identity.Authorize(ctx, actor, "", "manage_catalog", ""); err != nil {
		return nil, err
	}
	if gate.Purpose != "evidence" || gate.Status != "waiting" || gate.SubjectHash != hash {
		return nil, ErrConflict
	}
	return gate, nil
}

func lockRepairGate(ctx context.Context, tx pgx.Tx, actor identity.Actor, id string) (*Gate, error) {
	if actor.TokenID != "" {
		return nil, identity.ErrForbidden
	}
	if err := identity.Lock(ctx, tx, actor.Namespace); err != nil {
		return nil, err
	}
	gate, err := currentGate(ctx, tx, actor.Namespace, id)
	if err != nil {
		return nil, err
	}
	if err = identity.Authorize(ctx, actor, gate.ProjectID, "manage_run", gate.OwnerID); err != nil {
		return nil, err
	}
	if err = identity.Authorize(ctx, actor, "", "manage_catalog", ""); err != nil {
		return nil, err
	}
	if err = lockActiveRun(ctx, tx, gate.RunID); err != nil {
		return nil, err
	}
	if err = checkHead(ctx, tx, gate); err != nil {
		return nil, err
	}
	return gate, nil
}

func DecideRepair(ctx context.Context, actor identity.Actor, id string, input Decision) (*Gate, error) {
	if input.Decision != "approved" || input.Reason != "" || len(input.Items) > 0 {
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
	gate, err := lockRepairGate(ctx, tx, actor, id)
	if err != nil {
		return nil, err
	}
	r, body, err := readRepair(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if r == nil || gate.Purpose != "evidence" || gate.SubjectHash != input.SubjectHash || r.Hash != input.RepairHash || body.SubjectHash != input.SubjectHash {
		return nil, ErrConflict
	}
	if (r.Status == "approved" || r.Status == "consumed") && gate.ActorID == actor.ID {
		return Get(ctx, actor.Namespace, id)
	}
	if gate.Status != "waiting" || r.Status != "prepared" {
		return nil, ErrConflict
	}
	if err = catalog.CheckRepairDraft(ctx, tx, actor.Namespace, body.Execution); err != nil {
		return nil, err
	}
	if err = validateRepairFork(ctx, tx, gate, body); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.evidence_repairs SET status='approved' WHERE id=$1", r.ID); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.approvals SET status='approved',actor_id=$2,reason='批准修正版本并定向续跑；不追溯改变历史验收',items='[]',decided_at=now(),signalled_at=NULL WHERE id=$1", id, actor.ID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return Get(ctx, actor.Namespace, id)
}
