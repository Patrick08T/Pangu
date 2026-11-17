package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func loadRebuild(ctx context.Context, namespace, runID, intentID string) (*rebuildState, error) {
	actor := identity.Current(ctx)
	if actor.Namespace != namespace || actor.ID == "" {
		return nil, identity.ErrForbidden
	}
	run, err := uuid.Parse(runID)
	if err != nil {
		return nil, approval.ErrInvalid
	}
	intent, err := uuid.Parse(intentID)
	if err != nil {
		return nil, approval.ErrInvalid
	}
	previous, err := submission.ForRun(ctx, namespace, run.String())
	if err != nil {
		return nil, err
	}
	if err = authorizeRebuild(ctx, namespace, previous); err != nil {
		return nil, err
	}
	intents, err := approval.RebuildIntents(ctx, namespace, run.String())
	if err != nil {
		return nil, err
	}
	for _, item := range intents {
		if item.ID == intent.String() && item.RunID == previous.RunID {
			return &rebuildState{previous: previous, intent: &item}, nil
		}
	}
	return nil, approval.ErrNotFound
}

func authorizeRebuild(ctx context.Context, namespace string, previous *submission.Record) error {
	actor := identity.Current(ctx)
	if actor.ID == "" || actor.Namespace != namespace || actor.ID != previous.CreatedBy {
		return identity.ErrForbidden
	}
	if previous.ProjectID == "" || previous.Snapshot.Workflow.ProjectID != previous.ProjectID || previous.Snapshot.CreatedBy != actor.ID {
		return approval.ErrConflict
	}
	if purpose := previous.Snapshot.Purpose; purpose != "" && purpose != "business" {
		return approval.ErrConflict
	}
	for _, action := range []string{"run", "manage_run"} {
		if err := identity.Authorize(ctx, actor, previous.ProjectID, action, previous.CreatedBy); err != nil {
			return err
		}
	}
	return checkRebuildToken(ctx, actor, previous.ProjectID)
}

func checkRebuildToken(ctx context.Context, actor identity.Actor, project string) error {
	if actor.TokenID == "" {
		return nil
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var valid bool
	err = conn.QueryRow(ctx, `SELECT true FROM pangu.api_tokens t JOIN pangu.users u ON u.id=t.user_id
 WHERE t.id=$1 AND t.user_id=$2 AND t.project_id=$3 AND u.namespace=$4 AND NOT u.disabled
 AND t.revoked_at IS NULL AND t.expires_at>now() AND t.scopes @> ARRAY['run','manage_run']::text[]`, actor.TokenID, actor.ID, project, actor.Namespace).Scan(&valid)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !valid {
		return identity.ErrForbidden
	}
	return err
}
