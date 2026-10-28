package factory

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func Maintain(ctx context.Context, ns string) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT s.id::text FROM pangu.factory_sessions s WHERE namespace=$1
 AND NOT EXISTS(SELECT 1 FROM pangu.factory_turns active WHERE active.session_id=s.id AND active.status='pending')
 AND (updated_at<now()-interval '90 days' OR EXISTS(SELECT 1 FROM pangu.factory_turns t WHERE t.session_id=s.id AND t.finished_at<now()-interval '30 days' AND t.status NOT IN ('pending','expired')))
 ORDER BY updated_at LIMIT 100`, ns)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err = expireSession(ctx, conn, ns, id); err != nil {
			return err
		}
	}
	return nil
}

func expireSession(ctx context.Context, conn *pgx.Conn, ns, id string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT id FROM pangu.factory_sessions WHERE namespace=$1 AND id=$2 FOR UPDATE", ns, id); err != nil {
		return err
	}
	var pending bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.factory_turns WHERE session_id=$1 AND status='pending')", id).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return nil
	}
	changed, err := tx.Exec(ctx, `UPDATE pangu.factory_turns SET input=input||jsonb_build_object('message','正文已清理'),reply=NULL,
 result=CASE WHEN result IS NULL THEN NULL ELSE
 jsonb_build_object('agents',(SELECT jsonb_agg(a||'{"spec":{}}'::jsonb) FROM jsonb_array_elements(result->'agents') a)) ||
 CASE WHEN result ? 'workflow' THEN jsonb_build_object('workflow',(result->'workflow')||'{"spec":{}}'::jsonb) ELSE '{}'::jsonb END END,
 error='',status='expired' WHERE session_id=$1 AND status NOT IN ('pending','expired') AND finished_at<now()-interval '30 days'`, id)
	if err != nil {
		return err
	}
	if changed.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, `UPDATE pangu.factory_sessions SET revision=revision+1,title=CASE WHEN (SELECT status='expired' FROM pangu.factory_turns WHERE session_id=$1 ORDER BY sequence LIMIT 1) THEN '历史对话（正文已清理）' ELSE title END WHERE id=$1`, id)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, "DELETE FROM pangu.factory_sessions WHERE id=$1 AND updated_at<now()-interval '90 days'", id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
