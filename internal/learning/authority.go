package learning

import (
	"context"
	"errors"
	"slices"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func authorize(ctx context.Context, tx pgx.Tx, scope experience.Scope) error {
	action := permission(scope.Action)
	actor := identity.Current(ctx)
	if action == "" || scope.Namespace == "" || !validID(scope.ProjectID) || !validID(scope.ActorID) ||
		(actor.ID != "" && (actor.ID != scope.ActorID || actor.Namespace != scope.Namespace)) {
		return experience.ErrForbidden
	}
	if err := identity.Lock(ctx, tx, scope.Namespace); err != nil {
		return err
	}
	var admin, exists bool
	var role string
	err := tx.QueryRow(ctx, `SELECT u.is_admin, EXISTS(SELECT 1 FROM pangu.projects p
	 WHERE p.id=$3 AND p.namespace=u.namespace), coalesce((SELECT m.role FROM pangu.memberships m
	 WHERE m.user_id=u.id AND m.project_id=$3 AND m.enabled),'') FROM pangu.users u
	 WHERE u.id=$1 AND u.namespace=$2 AND NOT u.disabled`, scope.ActorID, scope.Namespace, scope.ProjectID).Scan(&admin, &exists, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return experience.ErrForbidden
	}
	if err != nil {
		return err
	}
	if !exists {
		return experience.ErrForbidden
	}
	if admin {
		role = "manager"
	}
	if role != "manager" && role != "operator" && role != "viewer" {
		return experience.ErrForbidden
	}
	if action == "manage" && (role != "manager" || actor.TokenID != "") || action == "run" && role == "viewer" {
		return experience.ErrForbidden
	}
	if actor.ID == "" {
		return authorizeBackground(ctx, tx, scope, action)
	}
	if actor.TokenID != "" {
		return authorizeToken(ctx, tx, scope, actor, action)
	}
	return nil
}

func permission(action string) string {
	switch action {
	case "policy_write", "version_activate", "delivery_change", "version_revoke", "source_write", "checker_write", "claim_check":
		return "manage"
	case "policy_read", "version_read", "version_list", "usage_list", "lineage_read", "watermark_read", "artifact_read", "reference_read", "source_read", "checker_read", "receipt_read":
		return "read"
	case "version_create", "watermark_freeze", "select", "artifact_write", "intent_write", "intent_complete", "intent_retry", "observation_write":
		return "run"
	}
	return ""
}

func validID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func authorizeToken(ctx context.Context, tx pgx.Tx, scope experience.Scope, actor identity.Actor, action string) error {
	if !validID(actor.TokenID) || actor.ProjectID != scope.ProjectID || !slices.Contains(actor.Scopes, action) {
		return experience.ErrForbidden
	}
	var scopes []string
	err := tx.QueryRow(ctx, `SELECT scopes FROM pangu.api_tokens WHERE id=$1 AND user_id=$2
	 AND project_id=$3 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`,
		actor.TokenID, scope.ActorID, scope.ProjectID).Scan(&scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return experience.ErrForbidden
	}
	if err != nil {
		return err
	}
	if !slices.Contains(scopes, action) {
		return experience.ErrForbidden
	}
	return nil
}

func authorizeBackground(ctx context.Context, tx pgx.Tx, scope experience.Scope, action string) error {
	if slices.Contains([]string{"policy_write", "source_write", "checker_write"}, scope.Action) {
		return experience.ErrForbidden
	}
	var allowed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.experience_policies
	 WHERE namespace=$1 AND project_id=$2 AND actor_id=$3::text) OR ($4<>'manage' AND EXISTS(
	 SELECT 1 FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND created_by::text=$3::text))`,
		scope.Namespace, scope.ProjectID, scope.ActorID, action).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return experience.ErrForbidden
	}
	return nil
}
