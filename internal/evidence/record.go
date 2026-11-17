package evidence

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func (b *Bundle) records(ctx context.Context, tx pgx.Tx, run string) error {
	queries := []struct{ kind, title, sql string }{
		{"repairs", "修正方案、决定与实际发布版本（仅 consumed 表示已生效）", `SELECT coalesce(jsonb_agg(jsonb_build_object('id',r.id,'approval_id',r.approval_id,'subject_hash',r.subject_hash,'status',r.status,'created_by',r.created_by,'created_at',r.created_at,'actor_id',a.actor_id,'decided_at',a.decided_at,'consumed_at',a.consumed_at,'proposal',r.body-'fork','reuse',(r.body->'fork')-'reused','version_id',c.version_id,'request_id',c.request_id) ORDER BY r.created_at,r.id),'[]'::jsonb) FROM pangu.evidence_repairs r JOIN pangu.approvals a ON a.id=r.approval_id LEFT JOIN pangu.plan_continuations c ON c.approval_id=a.id WHERE a.run_id=$1 AND r.body IS NOT NULL`},
		{"approvals", "人工决定、冻结主体及审批消费", `SELECT coalesce(jsonb_agg(jsonb_build_object('id',a.id,'run_id',a.run_id,'node_id',a.node_id,'round',a.round,'status',a.status,'actor_id',a.actor_id,'created_at',a.created_at,'decided_at',a.decided_at,'consumed_at',a.consumed_at,'subject_hash',a.subject_hash,'head_commit',a.head_commit,'reason',a.reason,'items',a.items,'plan_reports',(SELECT jsonb_object_agg(p.key,p.value-'text') FROM jsonb_each(a.subject#>'{node_input,parents}') p)) ORDER BY a.created_at,a.id),'[]'::jsonb) FROM pangu.approvals a WHERE a.run_id=$1`},
		{"checkpoints", "成功文件检查点与节点输出（命令另见尝试与动作）", `SELECT coalesce(jsonb_agg(to_jsonb(c)-'evidence' ORDER BY c.created_at,c.node_run_id),'[]'::jsonb) FROM pangu.workspace_checkpoints c WHERE c.run_id=$1`},
		{"usage", "实际调用与原始用量，null 保持未知", `SELECT coalesce(jsonb_agg(to_jsonb(u) ORDER BY u.created_at,u.id),'[]'::jsonb) FROM pangu.usage_calls u WHERE u.run_id=$1`},
	}
	for _, q := range queries {
		var raw []byte
		if err := tx.QueryRow(ctx, q.sql, run).Scan(&raw); err != nil {
			return err
		}
		if err := b.add(run, q.kind, q.title, q.kind+".json", string(raw)); err != nil {
			return err
		}
	}
	if err := b.attempts(ctx, tx, run); err != nil {
		return err
	}
	return b.reports(ctx, tx, run)
}

func (b *Bundle) attempts(ctx context.Context, tx pgx.Tx, run string) error {
	rows, err := tx.Query(ctx, `SELECT a.node_run_id::text||'-'||a.attempt::text,a.node_id,to_jsonb(a)-'owner_host'-'owner_pid'-'owner_stamp'-'process_scope'
 FROM pangu.node_attempts a WHERE a.run_id=$1 ORDER BY a.started_at,a.node_run_id,a.attempt`, run)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, node string
		var raw []byte
		if err = rows.Scan(&id, &node, &raw); err != nil {
			break
		}
		if err = b.add(run, "attempt", node+" 尝试与原始命令记录", "attempts/"+id+".json", string(raw)); err != nil {
			break
		}
	}
	readErr := rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if readErr != nil {
		return readErr
	}
	return b.actions(ctx, tx, run)
}

func (b *Bundle) actions(ctx context.Context, tx pgx.Tx, run string) error {
	rows, err := tx.Query(ctx, `SELECT e.node_run_id::text,jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM pangu.trace_events e WHERE e.run_id=$1 AND e.kind IN ('tool','node') GROUP BY e.node_run_id ORDER BY e.node_run_id`, run)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var node string
		var raw []byte
		if err = rows.Scan(&node, &raw); err != nil {
			return err
		}
		if err = b.add(run, "actions", "节点与工具动作追踪（含截断标记）", "actions/"+node+".json", string(raw)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (b *Bundle) reports(ctx context.Context, tx pgx.Tx, run string) error {
	rows, err := tx.Query(ctx, `SELECT id::text,title,body FROM pangu.research_reports WHERE run_id=$1 ORDER BY created_at,id`, run)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, title string
		var raw []byte
		if err = rows.Scan(&id, &title, &raw); err != nil {
			return err
		}
		if err = b.add(run, "report", title, "reports/"+id+".txt", string(raw)); err != nil {
			return err
		}
	}
	return rows.Err()
}
