package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type Draft struct {
	ID          string               `json:"id"`
	ProjectID   string               `json:"project_id"`
	ActorID     string               `json:"actor_id"`
	SourceRunID string               `json:"source_run_id"`
	ReportID    string               `json:"report_id"`
	ReportHash  string               `json:"report_hash"`
	SubjectHash string               `json:"subject_hash"`
	Definition  *workflow.Definition `json:"definition"`
	RequestID   string               `json:"request_id"`
	CreatedAt   time.Time            `json:"created_at"`
	ExpiresAt   time.Time            `json:"expires_at"`
}

const draftColumns = `d.id::text,d.project_id::text,d.actor_id::text,d.source_run_id::text,d.report_id::text,d.report_hash,d.subject_hash,
 CASE WHEN d.expires_at<=clock_timestamp() OR NOT EXISTS(SELECT 1 FROM pangu.research_reports r JOIN pangu.workspace_runs w ON w.run_id=r.run_id WHERE r.id=d.report_id AND w.expired_at IS NULL) THEN NULL ELSE d.definition END,
 coalesce(d.request_id::text,(SELECT s.id::text FROM pangu.submissions s WHERE s.namespace=d.namespace AND s.project_id=d.project_id AND s.operation='submit' AND s.payload_hash=d.submission_hash AND s.key_hash=encode(sha256(convert_to('delivery:'||d.id::text,'UTF8')),'hex')),''),d.created_at,d.expires_at`

func scanDraft(row pgx.Row) (*Draft, error) {
	out := &Draft{}
	var raw []byte
	err := row.Scan(&out.ID, &out.ProjectID, &out.ActorID, &out.SourceRunID, &out.ReportID, &out.ReportHash, &out.SubjectHash, &raw, &out.RequestID, &out.CreatedAt, &out.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		err = json.Unmarshal(raw, &out.Definition)
	}
	return out, err
}

func hash(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func Create(ctx context.Context, namespace, project, actor, run, reportID, key string, input CreateInput) (*Draft, error) {
	payloadHash, err := hash([]any{project, actor, run, reportID, input})
	if err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	existing, err := scanDraft(conn.QueryRow(ctx, "SELECT "+draftColumns+" FROM pangu.deliveries d WHERE d.namespace=$1 AND d.request_key=$2 AND d.payload_hash=$3", namespace, key, payloadHash))
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	report, err := research.ReadDetail(ctx, namespace, run, reportID)
	if err != nil {
		return nil, err
	}
	impl, err := catalog.ReadVersion(ctx, namespace, input.ImplementationVersion)
	if err != nil {
		return nil, err
	}
	review, err := catalog.ReadVersion(ctx, namespace, input.ReviewVersion)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	definition, err := Build(id, run, project, input, *report, *impl, *review)
	if err != nil {
		return nil, err
	}
	subjectHash, err := hash(definition)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(definition)
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.deliveries(id,namespace,project_id,actor_id,source_run_id,report_id,report_hash,request_key,payload_hash,subject_hash,definition)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(namespace,request_key) DO NOTHING`, id, namespace, project, actor, run, reportID, report.SubjectHash, key, payloadHash, subjectHash, raw)
	if err != nil {
		return nil, err
	}
	out, err := scanDraft(conn.QueryRow(ctx, "SELECT "+draftColumns+" FROM pangu.deliveries d WHERE d.namespace=$1 AND d.request_key=$2 AND d.payload_hash=$3", namespace, key, payloadHash))
	if errors.Is(err, ErrNotFound) {
		return nil, ErrConflict
	}
	return out, err
}

func Get(ctx context.Context, namespace, id string) (*Draft, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scanDraft(conn.QueryRow(ctx, "SELECT "+draftColumns+" FROM pangu.deliveries d WHERE d.namespace=$1 AND d.id=$2", namespace, id))
}

func List(ctx context.Context, namespace, project string, offset int) ([]Draft, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+draftColumns+" FROM pangu.deliveries d WHERE d.namespace=$1 AND d.project_id=$2 ORDER BY d.created_at DESC,d.id DESC LIMIT 100 OFFSET $3", namespace, project, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Draft{}
	for rows.Next() {
		item, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func Link(ctx context.Context, namespace, id, request string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `UPDATE pangu.deliveries d SET request_id=s.id FROM pangu.submissions s WHERE d.namespace=$1 AND d.id=$2 AND s.id=$3 AND s.namespace=d.namespace AND s.project_id=d.project_id
 AND s.payload_hash=d.submission_hash AND (d.request_id IS NULL OR d.request_id=s.id)`, namespace, id, request)
	if err == nil && result.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}

func Claim(ctx context.Context, namespace string, draft *Draft, actor string) error {
	if draft.Definition == nil {
		return ErrExpired
	}
	expected, err := hash(submission.Snapshot{Workflow: *draft.Definition, CreatedBy: actor})
	if err != nil {
		return err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `UPDATE pangu.deliveries SET submission_hash=$3 WHERE namespace=$1 AND id=$2 AND (submission_hash IS NULL OR submission_hash=$3) AND definition IS NOT NULL AND expires_at>clock_timestamp()`, namespace, draft.ID, expected)
	if err == nil && result.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}

func Maintain(ctx context.Context, namespace string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `UPDATE pangu.deliveries d SET definition=NULL WHERE namespace=$1 AND (expires_at<=clock_timestamp() OR NOT EXISTS(SELECT 1 FROM pangu.research_reports r JOIN pangu.workspace_runs w ON w.run_id=r.run_id WHERE r.id=d.report_id AND w.expired_at IS NULL))`, namespace)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `DELETE FROM pangu.deliveries d WHERE namespace=$1 AND created_at<now()-interval '90 days' AND NOT EXISTS(SELECT 1 FROM pangu.submissions s WHERE s.id=d.request_id AND s.run_id IS NULL)`, namespace)
	return err
}
