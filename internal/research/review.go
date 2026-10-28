package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

var ErrNotFound = errors.New("报告或结论不存在，或正文已过期")
var ErrReviewConflict = errors.New("报告摘要或人工核验修订已变化")
var ErrReviewInvalid = errors.New("报告核验参数无效或内容被安全规则阻止")

type ClaimReviewInput struct {
	Revision    int    `json:"revision"`
	SubjectHash string `json:"subject_hash"`
	Verdict     string `json:"verdict"`
	Reason      string `json:"reason"`
}
type ClaimReview struct {
	Source string `json:"source"`
	ClaimReviewInput
	ClaimID   string    `json:"claim_id"`
	ActorID   string    `json:"actor_id"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Detail struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	SubjectHash string            `json:"subject_hash"`
	Data        *StructuredReport `json:"data"`
	Reviews     []ClaimReview     `json:"reviews"`
}

const detailQuery = `SELECT r.id::text,r.title,r.body,r.metadata,coalesce((SELECT jsonb_agg(jsonb_build_object('claim_id',v.claim_id,'actor_id',v.actor_id,'revision',v.revision,'subject_hash',v.subject_hash,'verdict',v.verdict,'reason',v.reason,'updated_at',v.updated_at,'source',v.source) ORDER BY v.claim_id) FROM pangu.claim_reviews v WHERE v.report_id=r.id),'[]'::jsonb)
 FROM pangu.research_reports r JOIN pangu.workspace_runs w ON w.run_id=r.run_id JOIN pangu.projects p ON p.id=w.project_id
 WHERE p.namespace=$1 AND r.run_id=$2 AND w.expired_at IS NULL`

func scanDetail(row pgx.Row) (*Detail, error) {
	out := &Detail{}
	var body, metadata, reviews []byte
	err := row.Scan(&out.ID, &out.Title, &body, &metadata, &reviews)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if diagnostic.Sensitive(body) || diagnostic.Sensitive(metadata) {
		return nil, ErrReviewInvalid
	}
	data, err := json.Marshal(struct {
		Body     string
		Metadata json.RawMessage
	}{string(body), metadata})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	out.SubjectHash = hex.EncodeToString(hash[:])
	if len(metadata) > 0 {
		if err = json.Unmarshal(metadata, &out.Data); err != nil {
			return nil, err
		}
	}
	return out, json.Unmarshal(reviews, &out.Reviews)
}

func Details(ctx context.Context, namespace, run string) ([]Detail, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, detailQuery+" ORDER BY r.created_at,r.id LIMIT 32", namespace, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Detail{}
	for rows.Next() {
		item, err := scanDetail(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func ReadDetail(ctx context.Context, namespace, run, id string) (*Detail, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scanDetail(conn.QueryRow(ctx, detailQuery+" AND r.id=$3", namespace, run, id))
}

func (input ClaimReviewInput) Validate() error {
	hash, err := hex.DecodeString(input.SubjectHash)
	if err != nil || len(hash) != 32 || input.Revision < 0 || input.Revision > 2147483646 || strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 2048 || diagnostic.Sensitive([]byte(input.Reason)) {
		return ErrReviewInvalid
	}
	switch input.Verdict {
	case "supported", "partial", "unsupported", "outdated":
		return nil
	}
	return ErrReviewInvalid
}

func Review(ctx context.Context, namespace, run, report, claim string, actor identity.Actor, input ClaimReviewInput) (*ClaimReview, error) {
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
	if err = authorizeReview(ctx, tx, namespace, run, actor); err != nil {
		return nil, err
	}
	item, err := scanDetail(tx.QueryRow(ctx, detailQuery+" AND r.id=$3", namespace, run, report))
	if err != nil {
		return nil, err
	}
	if item.SubjectHash != input.SubjectHash {
		return nil, ErrReviewConflict
	}
	found := false
	if item.Data != nil {
		for _, c := range item.Data.Claims {
			found = found || c.ID == claim
		}
	}
	if !found {
		return nil, ErrNotFound
	}
	out, err := writeReview(ctx, tx, report, claim, actor.ID, input)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func writeReview(ctx context.Context, tx pgx.Tx, report, claim, actor string, input ClaimReviewInput) (*ClaimReview, error) {
	columns := "revision,subject_hash,verdict,reason,claim_id,actor_id::text,updated_at,source"
	row := tx.QueryRow(ctx, `INSERT INTO pangu.claim_reviews(report_id,claim_id,actor_id,subject_hash,verdict,reason,source)
 SELECT $1,$2,$3,$4,$5,$6,'feishu_session' WHERE $7=0 OR EXISTS(SELECT 1 FROM pangu.claim_reviews WHERE report_id=$1 AND claim_id=$2)
 ON CONFLICT(report_id,claim_id) DO UPDATE SET actor_id=excluded.actor_id,revision=pangu.claim_reviews.revision+1,verdict=excluded.verdict,reason=excluded.reason,updated_at=now(),source=excluded.source
 WHERE pangu.claim_reviews.revision=$7 AND pangu.claim_reviews.subject_hash=$4 RETURNING `+columns, report, claim, actor, input.SubjectHash, input.Verdict, input.Reason, input.Revision)
	out, err := scanReview(row)
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	out, err = scanReview(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.claim_reviews WHERE report_id=$1 AND claim_id=$2", report, claim))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil && out.Source == "feishu_session" && out.ActorID == actor && out.SubjectHash == input.SubjectHash && out.Verdict == input.Verdict && out.Reason == input.Reason {
		return out, nil
	}
	return nil, ErrReviewConflict
}

func scanReview(row pgx.Row) (*ClaimReview, error) {
	out := &ClaimReview{}
	err := row.Scan(&out.Revision, &out.SubjectHash, &out.Verdict, &out.Reason, &out.ClaimID, &out.ActorID, &out.UpdatedAt, &out.Source)
	return out, err
}

func authorizeReview(ctx context.Context, tx pgx.Tx, namespace, run string, actor identity.Actor) error {
	if actor.ID == "" || actor.TokenID != "" || actor.Namespace != namespace {
		return identity.ErrForbidden
	}
	if err := identity.Lock(ctx, tx, namespace); err != nil {
		return err
	}
	var project, owner string
	if err := tx.QueryRow(ctx, "SELECT project_id::text,coalesce(created_by::text,'') FROM pangu.submissions WHERE namespace=$1 AND run_id=$2", namespace, run).Scan(&project, &owner); err != nil {
		return err
	}
	return identity.Authorize(ctx, actor, project, "manage_run", owner)
}
