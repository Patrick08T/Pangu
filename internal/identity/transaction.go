package identity

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// 权限变更和新计划/触发按命名空间排序，已接受的触发继续执行。
func Lock(ctx context.Context, tx pgx.Tx, namespace string) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "identity:"+namespace)
	return err
}

func ScheduleOwner(ctx context.Context, tx pgx.Tx, user, project string) (bool, error) {
	var allowed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.users u JOIN pangu.projects p ON p.namespace=u.namespace
 WHERE u.id=$1 AND p.id=$2 AND NOT u.disabled AND (u.is_admin OR EXISTS(SELECT 1 FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=p.id AND m.enabled AND m.role IN ('operator','manager'))))`, user, project).Scan(&allowed)
	return allowed, err
}
