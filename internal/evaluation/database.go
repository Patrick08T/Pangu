package evaluation

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("评价或样本不存在")
var ErrInvalid = errors.New("评价或样本参数无效")
var ErrConflict = errors.New("评价版本或产物已变化，请重新读取")
var ErrExpired = errors.New("评价正文或样本已过期")

type FeedbackInput struct {
	Revision    int    `json:"revision"`
	SubjectHash string `json:"subject_hash"`
	Decision    string `json:"decision"`
	Reason      string `json:"reason"`
}

type Feedback struct {
	Source  string `json:"source"`
	ID      string `json:"id"`
	RunID   string `json:"run_id"`
	TaskID  string `json:"task_id"`
	ActorID string `json:"actor_id"`
	FeedbackInput
	FirstDecision  string    `json:"first_decision"`
	PayloadExpired bool      `json:"payload_expired"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

const feedbackColumns = `id::text,run_id::text,task_id::text,actor_id::text,revision,subject_hash,decision,reason,first_decision,payload_expired,created_at,updated_at,source`

func (input FeedbackInput) Validate() error {
	hash, err := hex.DecodeString(input.SubjectHash)
	if err != nil || len(hash) != 32 || input.Revision < 0 || input.Revision > 2147483646 {
		return ErrInvalid
	}
	if input.Decision != "accepted" && input.Decision != "needs_changes" && input.Decision != "rejected" {
		return ErrInvalid
	}
	if strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 2048 || diagnostic.Sensitive([]byte(input.Reason)) {
		return ErrInvalid
	}
	return nil
}

func scanFeedback(row pgx.Row) (*Feedback, error) {
	out := &Feedback{}
	err := row.Scan(&out.ID, &out.RunID, &out.TaskID, &out.ActorID, &out.Revision, &out.SubjectHash, &out.Decision, &out.Reason, &out.FirstDecision, &out.PayloadExpired, &out.CreatedAt, &out.UpdatedAt, &out.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

func FeedbackForRun(ctx context.Context, namespace, run string) (*Feedback, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scanFeedback(conn.QueryRow(ctx, "SELECT "+feedbackColumns+" FROM pangu.feedback WHERE namespace=$1 AND run_id=$2", namespace, run))
}

func SaveFeedback(ctx context.Context, namespace, project, run string, actor identity.Actor, input FeedbackInput) (*Feedback, error) {
	if err := input.Validate(); err != nil {
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
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(hashtextextended($1,0))", "retention:"+run); err != nil {
		return nil, err
	}
	if err = authorizeFeedback(ctx, tx, namespace, project, run, actor); err != nil {
		return nil, err
	}
	var expired bool
	err = tx.QueryRow(ctx, `SELECT s.purged_at IS NOT NULL OR NOT(s.snapshot->'workflow' ? 'input') OR coalesce(f.payload_expired,false)
 FROM pangu.submissions s LEFT JOIN pangu.feedback f ON f.run_id=s.run_id
 WHERE s.namespace=$1 AND s.project_id=$2 AND s.run_id=$3`, namespace, project, run).Scan(&expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, ErrExpired
	}
	item, err := writeFeedback(ctx, tx, namespace, project, run, actor.ID, input)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit(ctx)
}

func authorizeFeedback(ctx context.Context, tx pgx.Tx, namespace, project, run string, actor identity.Actor) error {
	if actor.ID == "" || actor.TokenID != "" || actor.Namespace != namespace {
		return identity.ErrForbidden
	}
	if err := identity.Lock(ctx, tx, namespace); err != nil {
		return err
	}
	var owner string
	if err := tx.QueryRow(ctx, "SELECT coalesce(created_by::text,'') FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND run_id=$3", namespace, project, run).Scan(&owner); err != nil {
		return err
	}
	return identity.Authorize(ctx, actor, project, "manage_run", owner)
}

func writeFeedback(ctx context.Context, tx pgx.Tx, namespace, project, run, actor string, input FeedbackInput) (*Feedback, error) {
	item, err := scanFeedback(tx.QueryRow(ctx, `INSERT INTO pangu.feedback(id,namespace,project_id,run_id,task_id,actor_id,subject_hash,decision,first_decision,reason,source)
 SELECT $1,s.namespace,s.project_id,s.run_id,s.task_id,$5,$6,$7,$7,$8,'feishu_session' FROM pangu.submissions s
 WHERE s.namespace=$2 AND s.project_id=$3 AND s.run_id=$4 AND ($9=0 OR EXISTS(SELECT 1 FROM pangu.feedback WHERE run_id=$4))
 ON CONFLICT(run_id) DO UPDATE SET actor_id=excluded.actor_id,decision=excluded.decision,reason=excluded.reason,revision=pangu.feedback.revision+1,updated_at=now(),source=excluded.source,
 first_decision=CASE WHEN pangu.feedback.source='feishu_session' THEN pangu.feedback.first_decision ELSE excluded.first_decision END
 WHERE pangu.feedback.revision=$9 AND pangu.feedback.subject_hash=$6 AND NOT pangu.feedback.payload_expired
 RETURNING `+feedbackColumns, uuid.NewString(), namespace, project, run, actor, input.SubjectHash, input.Decision, input.Reason, input.Revision))
	if !errors.Is(err, ErrNotFound) {
		return item, err
	}
	item, err = scanFeedback(tx.QueryRow(ctx, "SELECT "+feedbackColumns+" FROM pangu.feedback WHERE namespace=$1 AND run_id=$2", namespace, run))
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err == nil && item.Source == "feishu_session" && item.SubjectHash == input.SubjectHash && item.ActorID == actor && item.Decision == input.Decision && item.Reason == input.Reason && !item.PayloadExpired {
		return item, nil
	}
	return nil, ErrConflict
}
