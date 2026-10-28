package evaluation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CaseInput struct {
	Name        string   `json:"name"`
	Input       string   `json:"input"`
	Expected    string   `json:"expected"`
	Criteria    []string `json:"criteria"`
	RetainInput bool     `json:"retain_input"`
}
type Case struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	ActorID        string    `json:"actor_id"`
	Name           string    `json:"name"`
	Input          string    `json:"input"`
	Expected       string    `json:"expected"`
	Criteria       []string  `json:"criteria"`
	PayloadExpired bool      `json:"payload_expired"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

const caseColumns = `id::text,project_id::text,actor_id::text,
 CASE WHEN archived OR expires_at<=now() THEN '' ELSE name END,
 CASE WHEN archived OR expires_at<=now() THEN '' ELSE input END,
 CASE WHEN archived OR expires_at<=now() THEN '' ELSE expected END,
 CASE WHEN archived OR expires_at<=now() THEN '[]'::jsonb ELSE criteria END,
 archived OR expires_at<=now(),created_at,expires_at`

func (input CaseInput) Validate() error {
	if !input.RetainInput || len(input.Name) > 128 || strings.TrimSpace(input.Name) == "" || len(input.Input) > 16384 || len(input.Expected) > 8192 || strings.TrimSpace(input.Expected) == "" || len(input.Criteria) < 1 || len(input.Criteria) > 20 {
		return ErrInvalid
	}
	for _, item := range input.Criteria {
		if len(item) > 1024 || strings.TrimSpace(item) == "" {
			return ErrInvalid
		}
	}
	data, _ := json.Marshal(input)
	if diagnostic.Sensitive(data) {
		return ErrInvalid
	}
	return nil
}

func CreateCase(ctx context.Context, namespace, project, actor string, input CaseInput) (*Case, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	criteria, err := json.Marshal(input.Criteria)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(input.Input))
	return scanCase(conn.QueryRow(ctx, `INSERT INTO pangu.evaluation_cases(id,namespace,project_id,actor_id,name,input,input_hash,expected,criteria)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+caseColumns, uuid.NewString(), namespace, project, actor, input.Name, input.Input, hex.EncodeToString(hash[:]), input.Expected, criteria))
}

func scanCase(row pgx.Row) (*Case, error) {
	out := &Case{}
	var criteria []byte
	err := row.Scan(&out.ID, &out.ProjectID, &out.ActorID, &out.Name, &out.Input, &out.Expected, &criteria, &out.PayloadExpired, &out.CreatedAt, &out.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, json.Unmarshal(criteria, &out.Criteria)
}

func ReadCase(ctx context.Context, namespace, id string) (*Case, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scanCase(conn.QueryRow(ctx, "SELECT "+caseColumns+" FROM pangu.evaluation_cases WHERE namespace=$1 AND id=$2", namespace, id))
}

func Cases(ctx context.Context, namespace, project string, offset int) ([]Case, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+caseColumns+" FROM pangu.evaluation_cases WHERE namespace=$1 AND project_id=$2 ORDER BY created_at DESC,id DESC LIMIT 100 OFFSET $3", namespace, project, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Case{}
	for rows.Next() {
		item, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func RemoveCase(ctx context.Context, namespace, id string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, "UPDATE pangu.evaluation_cases SET archived=true,name='',input='',expected='',criteria='[]'::jsonb WHERE namespace=$1 AND id=$2", namespace, id)
	if err == nil && result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func Maintain(ctx context.Context, namespace string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "UPDATE pangu.evaluation_cases SET name='',input='',expected='',criteria='[]'::jsonb WHERE namespace=$1 AND (archived OR expires_at<=now())", namespace)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `DELETE FROM pangu.evaluation_cases c WHERE c.namespace=$1 AND c.created_at<now()-interval '90 days'
 AND NOT EXISTS(SELECT 1 FROM pangu.evaluation_runs e JOIN pangu.submissions s ON e.request_id=s.id WHERE e.case_id=c.id AND s.run_id IS NULL)`, namespace)
	return err
}
