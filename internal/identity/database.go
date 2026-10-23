package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Actor struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Admin     bool     `json:"admin"`
	CSRF      string   `json:"csrf,omitempty"`
	Namespace string   `json:"-"`
	TokenID   string   `json:"-"`
	ProjectID string   `json:"-"`
	Scopes    []string `json:"-"`
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func login(ctx context.Context, namespace string, cfg Config, external FeishuUser) (Actor, string, error) {
	if external.Tenant != cfg.Tenant || external.OpenID == "" {
		return Actor{}, "", ErrForbidden
	}
	token, err := randomToken()
	if err != nil {
		return Actor{}, "", err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return Actor{}, "", err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return Actor{}, "", err
	}
	defer tx.Rollback(context.Background())
	if cfg.AdminOpenID != "" && external.OpenID == cfg.AdminOpenID {
		_, err = tx.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1,$2,$3,$4,$5,true) ON CONFLICT(namespace,app_id,tenant_key,open_id) DO NOTHING`, uuid.NewString(), namespace, cfg.AppID, cfg.Tenant, external.OpenID)
		if err != nil {
			return Actor{}, "", err
		}
	}
	actor := Actor{Namespace: namespace, CSRF: digest("pangu-csrf:" + token)}
	err = tx.QueryRow(ctx, `UPDATE pangu.users u SET name=$5 WHERE namespace=$1 AND app_id=$2 AND tenant_key=$3 AND open_id=$4 AND NOT disabled
 AND (is_admin OR EXISTS(SELECT 1 FROM pangu.memberships m JOIN pangu.projects p ON p.id=m.project_id WHERE m.user_id=u.id AND m.enabled AND p.namespace=u.namespace)) RETURNING u.id::text,u.name,u.is_admin`, namespace, cfg.AppID, cfg.Tenant, external.OpenID, external.Name).Scan(&actor.ID, &actor.Name, &actor.Admin)
	if errors.Is(err, pgx.ErrNoRows) {
		return Actor{}, "", ErrForbidden
	}
	if err != nil {
		return Actor{}, "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '12 hours')`, digest(token), actor.ID)
	if err != nil {
		return Actor{}, "", err
	}
	return actor, token, tx.Commit(ctx)
}

func sessionActor(ctx context.Context, namespace string, cfg Config, token string) (Actor, error) {
	if len(token) != 43 {
		return Actor{}, ErrUnauthenticated
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return Actor{}, err
	}
	defer conn.Close(context.Background())
	actor := Actor{Namespace: namespace, CSRF: digest("pangu-csrf:" + token)}
	err = conn.QueryRow(ctx, `SELECT u.id::text,u.name,u.is_admin FROM pangu.sessions s JOIN pangu.users u ON u.id=s.user_id
 WHERE s.token_hash=$1 AND s.expires_at>now() AND u.namespace=$2 AND u.app_id=$3 AND u.tenant_key=$4 AND NOT u.disabled`, digest(token), namespace, cfg.AppID, cfg.Tenant).Scan(&actor.ID, &actor.Name, &actor.Admin)
	if errors.Is(err, pgx.ErrNoRows) {
		return Actor{}, ErrUnauthenticated
	}
	return actor, err
}

func logout(ctx context.Context, namespace, token string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `DELETE FROM pangu.sessions s USING pangu.users u WHERE s.user_id=u.id AND u.namespace=$1 AND s.token_hash=$2`, namespace, digest(token))
	return err
}
