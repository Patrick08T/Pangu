package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Query views intentionally exclude request bodies, suite data and machine receipts.
type QueryPage struct {
	AfterID string
	Limit   int
}
type IntentSummary struct {
	Kind                   string             `json:"kind"`
	ExperimentID           string             `json:"experiment_id,omitempty"`
	BenefitMemoryVersionID string             `json:"benefit_memory_version_id,omitempty"`
	ID                     string             `json:"id"`
	ProjectID              string             `json:"project_id"`
	PolicyRevision         int64              `json:"policy_revision"`
	Generation             int                `json:"generation"`
	State                  string             `json:"state"`
	Publication            *PublishedVersions `json:"publication,omitempty"`
	StartedAt              time.Time          `json:"started_at"`
}
type IntentPage struct {
	Items      []IntentSummary `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}
type EnvironmentPage struct {
	Items      []Environment `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

func queryGuard(ctx context.Context, tx pgx.Tx, scope Scope, page QueryPage) error {
	if page.Limit < 1 || page.Limit > 100 {
		return ErrInvalid
	}
	if page.AfterID != "" {
		if _, err := uuid.Parse(page.AfterID); err != nil {
			return ErrInvalid
		}
	}
	actor := identity.Current(ctx)
	if actor.ID != scope.ActorID || actor.Namespace != scope.Namespace || actor.TokenID != "" {
		return ErrForbidden
	}
	if err := lockScope(ctx, tx, scope); err != nil {
		return err
	}
	return managerTx(ctx, tx, scope)
}

func (s Service) ListIntents(ctx context.Context, scope Scope, page QueryPage) (IntentPage, error) {
	out := IntentPage{Items: []IntentSummary{}}
	err := transaction(ctx, func(tx pgx.Tx) error {
		if err := queryGuard(ctx, tx, scope, page); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id::text,project_id::text,policy_revision,generation,state,publication,created_at,coalesce(nullif(request#>>'{request,kind}',''),'role_change'),coalesce(experiment_id::text,''),coalesce(request#>>'{request,benefit_memory_version_id}','')
   FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2 AND ($3='' OR id>NULLIF($3,'')::uuid) ORDER BY id LIMIT $4`, scope.Namespace, scope.ProjectID, page.AfterID, page.Limit+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanIntentSummary(rows)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, item)
		}
		return rows.Err()
	})
	if err != nil {
		return IntentPage{}, err
	}
	if len(out.Items) > page.Limit {
		out.Items = out.Items[:page.Limit]
		out.NextCursor = out.Items[len(out.Items)-1].ID
	}
	return out, nil
}

func scanIntentSummary(row pgx.Row) (IntentSummary, error) {
	var item IntentSummary
	var publication []byte
	err := row.Scan(&item.ID, &item.ProjectID, &item.PolicyRevision, &item.Generation, &item.State, &publication, &item.StartedAt, &item.Kind, &item.ExperimentID, &item.BenefitMemoryVersionID)
	if err == nil && len(publication) > 0 {
		err = json.Unmarshal(publication, &item.Publication)
	}
	return item, err
}

func (s Service) ReadIntentSummary(ctx context.Context, scope Scope, id string) (IntentSummary, error) {
	var out IntentSummary
	err := transaction(ctx, func(tx pgx.Tx) error {
		if _, err := uuid.Parse(id); err != nil {
			return ErrInvalid
		}
		if err := queryGuard(ctx, tx, scope, QueryPage{Limit: 1}); err != nil {
			return err
		}
		var err error
		out, err = scanIntentSummary(tx.QueryRow(ctx, `SELECT id::text,project_id::text,policy_revision,generation,state,publication,created_at,coalesce(nullif(request#>>'{request,kind}',''),'role_change'),coalesce(experiment_id::text,''),coalesce(request#>>'{request,benefit_memory_version_id}','')
   FROM pangu.evolution_intents WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return out, err
}

func (s Service) ListEnvironments(ctx context.Context, scope Scope, page QueryPage) (EnvironmentPage, error) {
	out := EnvironmentPage{Items: []Environment{}}
	err := transaction(ctx, func(tx pgx.Tx) error {
		if err := queryGuard(ctx, tx, scope, page); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT spec,spec_hash FROM (SELECT DISTINCT ON(id) id,spec,spec_hash FROM pangu.evolution_environments
   WHERE namespace=$1 AND project_id=$2 AND ($3='' OR id>NULLIF($3,'')::uuid) ORDER BY id,revision DESC) e ORDER BY id LIMIT $4`, scope.Namespace, scope.ProjectID, page.AfterID, page.Limit+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Environment
			var body []byte
			var hash string
			if err = rows.Scan(&body, &hash); err != nil {
				return err
			}
			if err = json.Unmarshal(body, &item); err != nil {
				return err
			}
			if item.ProjectID != scope.ProjectID || hashJSON(item) != hash {
				return ErrConflict
			}
			out.Items = append(out.Items, item)
		}
		return rows.Err()
	})
	if err != nil {
		return EnvironmentPage{}, err
	}
	if len(out.Items) > page.Limit {
		out.Items = out.Items[:page.Limit]
		out.NextCursor = out.Items[len(out.Items)-1].ID
	}
	return out, nil
}
