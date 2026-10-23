package identity

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type Member struct {
	ID       string `json:"id"`
	OpenID   string `json:"open_id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Enabled  bool   `json:"enabled"`
	Disabled bool   `json:"disabled"`
}

func Members(ctx context.Context, actor Actor, project string) ([]Member, error) {
	if err := Authorize(ctx, actor, project, "manage_members", ""); err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT u.id::text,u.open_id,u.name,m.role,m.enabled,u.disabled FROM pangu.memberships m JOIN pangu.users u ON u.id=m.user_id WHERE m.project_id=$1 AND u.namespace=$2 ORDER BY u.created_at,u.id LIMIT 1000`, project, actor.Namespace)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Member])
}

type Membership struct {
	ProjectID string `json:"project_id"`
	Role      string `json:"role"`
}

func Memberships(ctx context.Context, actor Actor) ([]Membership, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT p.id::text,CASE WHEN u.is_admin THEN 'manager' ELSE m.role END
 FROM pangu.projects p JOIN pangu.users u ON u.namespace=p.namespace LEFT JOIN pangu.memberships m ON m.user_id=u.id AND m.project_id=p.id AND m.enabled
 WHERE u.id=$1 AND u.namespace=$2 AND NOT u.disabled AND (u.is_admin OR m.user_id IS NOT NULL) AND ($3='' OR p.id=nullif($3,'')::uuid) ORDER BY p.id`, actor.ID, actor.Namespace, actor.ProjectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Membership])
}

func Tokens(ctx context.Context, actor Actor) ([]Token, error) {
	if actor.ID == "" || actor.TokenID != "" {
		return nil, ErrForbidden
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT id::text,name,project_id::text,scopes,expires_at,revoked_at FROM pangu.api_tokens WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1000`, actor.ID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Token])
}
