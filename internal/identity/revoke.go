package identity

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
)

func RemoveMember(ctx context.Context, actor Actor, project, user string) error {
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
	if err = Lock(ctx, tx, actor.Namespace); err != nil {
		return err
	}
	if err = Authorize(ctx, actor, project, "manage_members", ""); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.memberships SET enabled=false WHERE project_id=$1 AND user_id=$2`, project, user)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrForbidden
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.api_tokens SET revoked_at=coalesce(revoked_at,now()) WHERE project_id=$1 AND user_id=$2`, project, user)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.schedules SET paused=true,revision=revision+1,remote_synced=false WHERE project_id=$1 AND created_by=$2 AND NOT paused AND NOT deleted`, project, user)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func Enable(ctx context.Context, actor Actor, user string) error {
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
	if err = Lock(ctx, tx, actor.Namespace); err != nil {
		return err
	}
	if err = Authorize(ctx, actor, "", "admin", ""); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.users SET disabled=false WHERE id=$1 AND namespace=$2`, user, actor.Namespace)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrForbidden
	}
	return tx.Commit(ctx)
}
