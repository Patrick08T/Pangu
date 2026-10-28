package identity

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Token struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	ProjectID string     `json:"project_id"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func IssueToken(ctx context.Context, actor Actor, project, name string, scopes []string, expires time.Time) (Token, string, error) {
	if actor.TokenID != "" {
		return Token{}, "", ErrForbidden
	}
	if strings.TrimSpace(name) == "" || len(name) > 128 || len(scopes) == 0 || len(scopes) > 8 || !expires.After(time.Now()) || expires.After(time.Now().Add(90*24*time.Hour)) {
		return Token{}, "", errors.Join(ErrInvalid, errors.New("Token 名称、权限或有效期无效；最长 90 天"))
	}
	if err := validateScopes(scopes); err != nil {
		return Token{}, "", err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return Token{}, "", err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return Token{}, "", err
	}
	defer tx.Rollback(context.Background())
	if err = Lock(ctx, tx, actor.Namespace); err != nil {
		return Token{}, "", err
	}
	for _, scope := range scopes {
		if err := Authorize(ctx, actor, project, scope, actor.ID); err != nil {
			return Token{}, "", err
		}
	}
	raw, err := randomToken()
	if err != nil {
		return Token{}, "", err
	}
	raw = "pgu_" + raw
	out := Token{ID: uuid.NewString(), Name: name, ProjectID: project, Scopes: scopes, ExpiresAt: expires.UTC()}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.api_tokens(id,token_hash,user_id,project_id,name,scopes,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, out.ID, digest(raw), actor.ID, project, name, scopes, out.ExpiresAt)
	if err != nil {
		return Token{}, "", err
	}
	return out, raw, tx.Commit(ctx)
}

func validateScopes(scopes []string) error {
	seen := map[string]bool{}
	for _, scope := range scopes {
		if seen[scope] || !slices.Contains([]string{"read", "run", "configure", "schedule", "manage_run", "manage_schedule", "manage_connections", "manage_factory"}, scope) {
			return errors.Join(ErrInvalid, errors.New("Token 权限无效或重复"))
		}
		seen[scope] = true
	}
	return nil
}

func tokenActor(ctx context.Context, namespace string, cfg Config, raw string) (Actor, error) {
	if len(raw) != 47 || !strings.HasPrefix(raw, "pgu_") {
		return Actor{}, ErrUnauthenticated
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return Actor{}, err
	}
	defer conn.Close(context.Background())
	out := Actor{Namespace: namespace}
	err = conn.QueryRow(ctx, `SELECT u.id::text,u.name,u.is_admin,t.id::text,t.project_id::text,t.scopes FROM pangu.api_tokens t JOIN pangu.users u ON u.id=t.user_id
 WHERE t.token_hash=$1 AND t.expires_at>now() AND t.revoked_at IS NULL AND u.namespace=$2 AND u.app_id=$3 AND u.tenant_key=$4 AND NOT u.disabled`, digest(raw), namespace, cfg.AppID, cfg.Tenant).Scan(&out.ID, &out.Name, &out.Admin, &out.TokenID, &out.ProjectID, &out.Scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Actor{}, ErrUnauthenticated
	}
	return out, err
}

func RevokeToken(ctx context.Context, actor Actor, id string) error {
	if actor.ID == "" || actor.TokenID != "" {
		return ErrForbidden
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, "UPDATE pangu.api_tokens SET revoked_at=coalesce(revoked_at,now()) WHERE id=$1 AND user_id=$2", id, actor.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrForbidden
	}
	return nil
}
