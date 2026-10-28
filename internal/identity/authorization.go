package identity

import (
	"context"
	"slices"

	"github.com/Patrick08T/Pangu/internal/audit"
	"github.com/Patrick08T/Pangu/internal/database"
)

type actorKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}
func Current(ctx context.Context) Actor { actor, _ := ctx.Value(actorKey{}).(Actor); return actor }

func Authorize(ctx context.Context, actor Actor, project, action, owner string) error {
	if actor.ID == "" || actor.Namespace == "" {
		return ErrUnauthenticated
	}
	if actor.TokenID != "" && (actor.ProjectID != project || !slices.Contains(actor.Scopes, action)) {
		return ErrForbidden
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var admin, exists bool
	var role string
	err = conn.QueryRow(ctx, `SELECT u.is_admin,
 CASE WHEN $3='' THEN EXISTS(SELECT 1 FROM pangu.memberships m JOIN pangu.projects p ON p.id=m.project_id WHERE m.user_id=u.id AND m.enabled AND m.role='manager' AND p.namespace=u.namespace)
 ELSE EXISTS(SELECT 1 FROM pangu.projects p WHERE p.id=nullif($3,'')::uuid AND p.namespace=u.namespace) END,
 coalesce((SELECT m.role FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=nullif($3,'')::uuid AND m.enabled),'')
 FROM pangu.users u WHERE u.id=$1 AND u.namespace=$2 AND NOT u.disabled`, actor.ID, actor.Namespace, project).Scan(&admin, &exists, &role)
	if err != nil {
		return ErrForbidden
	}
	if project == "" {
		if actor.TokenID != "" {
			return ErrForbidden
		}
		if action == "read_templates" {
			return nil
		}
		if action == "admin" && admin || action == "manage_templates" && (admin || exists) {
			return nil
		}
		return ErrForbidden
	}
	if !exists {
		return ErrForbidden
	}
	if admin {
		role = "manager"
	}
	if permits(role, action, owner, actor.ID) {
		audit.Project(ctx, project)
		return nil
	}
	return ErrForbidden
}

func permits(role, action, owner, user string) bool {
	if role != "manager" && role != "operator" && role != "viewer" {
		return false
	}
	switch action {
	case "read":
		return true
	case "run", "configure", "schedule":
		return role != "viewer"
	case "manage_run", "manage_schedule":
		return role == "manager" || role == "operator" && owner == user
	case "manage_connections", "manage_members":
		return role == "manager"
	}
	return false
}
