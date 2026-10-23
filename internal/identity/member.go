package identity

import (
	"context"
	"errors"
	"regexp"

	"github.com/Patrick08T/Pangu/internal/audit"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func Grant(ctx context.Context, actor Actor, project, openID, role string) (Actor, error) {
	if role != "manager" && role != "operator" && role != "viewer" || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(openID) {
		return Actor{}, errors.Join(ErrInvalid, errors.New("成员身份或角色无效"))
	}
	if err := Authorize(ctx, actor, project, "manage_members", ""); err != nil {
		return Actor{}, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return Actor{}, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return Actor{}, err
	}
	defer tx.Rollback(context.Background())
	if err = Lock(ctx, tx, actor.Namespace); err != nil {
		return Actor{}, err
	}
	if err = Authorize(ctx, actor, project, "manage_members", ""); err != nil {
		return Actor{}, err
	}
	var app, tenant string
	if err = tx.QueryRow(ctx, "SELECT app_id,tenant_key FROM pangu.users WHERE id=$1 AND namespace=$2", actor.ID, actor.Namespace).Scan(&app, &tenant); err != nil {
		return Actor{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(namespace,app_id,tenant_key,open_id) DO NOTHING`, uuid.NewString(), actor.Namespace, app, tenant, openID)
	if err != nil {
		return Actor{}, err
	}
	user := Actor{Namespace: actor.Namespace}
	err = tx.QueryRow(ctx, `SELECT id::text,name,is_admin FROM pangu.users WHERE namespace=$1 AND app_id=$2 AND tenant_key=$3 AND open_id=$4 AND NOT disabled FOR UPDATE`, actor.Namespace, app, tenant, openID).Scan(&user.ID, &user.Name, &user.Admin)
	if err != nil {
		return Actor{}, ErrForbidden
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.memberships(user_id,project_id,role) VALUES($1,$2,$3) ON CONFLICT(user_id,project_id) DO UPDATE SET role=$3,enabled=true`, user.ID, project, role)
	if err != nil {
		return Actor{}, err
	}
	if role == "viewer" {
		if _, err = tx.Exec(ctx, `UPDATE pangu.schedules SET paused=true,revision=revision+1,remote_synced=false WHERE created_by=$1 AND project_id=$2 AND NOT paused AND NOT deleted`, user.ID, project); err != nil {
			return Actor{}, err
		}
	}
	err = tx.Commit(ctx)
	if err == nil {
		audit.Membership(ctx, user.ID, role)
	}
	return user, err
}

func Disable(ctx context.Context, actor Actor, user string) error {
	if err := Authorize(ctx, actor, "", "admin", ""); err != nil {
		return err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "identity:"+actor.Namespace); err != nil {
		return err
	}
	if err = Authorize(ctx, actor, "", "admin", ""); err != nil {
		return err
	}
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT NOT is_admin OR (SELECT count(*) FROM pangu.users WHERE namespace=$2 AND is_admin AND NOT disabled)>1 FROM pangu.users WHERE id=$1 AND namespace=$2`, user, actor.Namespace).Scan(&allowed)
	if err != nil || !allowed {
		return ErrForbidden
	}
	for _, statement := range []string{`UPDATE pangu.users SET disabled=true WHERE id=$1`, `DELETE FROM pangu.sessions WHERE user_id=$1`, `UPDATE pangu.api_tokens SET revoked_at=coalesce(revoked_at,now()) WHERE user_id=$1`, `UPDATE pangu.schedules SET paused=true,revision=revision+1,remote_synced=false WHERE created_by=$1 AND NOT paused AND NOT deleted`} {
		if _, err = tx.Exec(ctx, statement, user); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
