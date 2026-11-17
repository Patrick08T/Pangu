package workspace

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func experienceErasureOutputSQL(column string) string {
	const body = `jsonb_strip_nulls(jsonb_build_object('experience_erased',true,
 'run_id',CASE WHEN VALUE->>'run_id' ~* '^[0-9a-f-]{36}$' THEN VALUE->'run_id' END,
 'report_ids',(SELECT jsonb_agg(v) FROM jsonb_array_elements_text(CASE WHEN jsonb_typeof(VALUE->'report_ids')='array' THEN VALUE->'report_ids' ELSE '[]'::jsonb END) v WHERE v ~* '^[0-9a-f-]{36}$'),
 'report_sha256',CASE WHEN VALUE->>'report_sha256' ~ '^[0-9a-f]{64}$' THEN VALUE->'report_sha256' END,
 'lineage',CASE WHEN VALUE#>>'{lineage,version}'='1' AND VALUE#>>'{lineage,artifact_id}' ~* '^[0-9a-f-]{36}$'
 AND VALUE#>>'{lineage,input_hash}' ~ '^[0-9a-f]{64}$' AND VALUE#>>'{lineage,output_hash}' ~ '^[0-9a-f]{64}$'
 AND VALUE#>>'{lineage,execution_input_hash}' ~ '^[0-9a-f]{64}$'
 THEN jsonb_build_object('version',1,'artifact_id',VALUE#>'{lineage,artifact_id}','input_hash',VALUE#>'{lineage,input_hash}',
 'output_hash',VALUE#>'{lineage,output_hash}','execution_input_hash',VALUE#>'{lineage,execution_input_hash}') END))`
	return strings.ReplaceAll(body, "VALUE", column)
}

func experienceErasureClear(ctx context.Context, conn *pgx.Conn, s experienceErasureScope, verify ExperienceErasureVerifier) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = experienceErasureVerifyTx(ctx, tx, s, verify); err != nil {
		return err
	}
	queries := []string{
		`UPDATE pangu.workspace_checkpoints SET output=` + experienceErasureOutputSQL("output") + `,evidence='[]'::jsonb,evidence_truncated=true WHERE run_id=$1`,
		`UPDATE pangu.claim_reviews c SET reason='' FROM pangu.research_reports r WHERE c.report_id=r.id AND r.run_id=$1`,
		`UPDATE pangu.research_reports SET title='',body=''::bytea,output=` + experienceErasureOutputSQL("output") + `,evidence='[]'::jsonb,metadata=NULL WHERE run_id=$1`,
		`UPDATE pangu.node_attempts SET message='',truncated=true,evidence='[]'::jsonb,evidence_truncated=true WHERE run_id=$1`,
		`UPDATE pangu.schedules p SET active_slot=NULL FROM pangu.schedule_fires f,pangu.submissions s WHERE p.id=f.schedule_id AND p.active_slot=f.slot AND f.request_id=s.id AND s.run_id=$1`,
		`UPDATE pangu.workspace_runs SET purged_at=coalesce(purged_at,clock_timestamp()) WHERE run_id=$1`,
	}
	for _, query := range queries {
		if _, err = tx.Exec(ctx, query, s.run); err != nil {
			return err
		}
	}
	if err = purgePayloads(ctx, tx, s.run); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `WITH done AS (UPDATE pangu.submissions SET purged_at=clock_timestamp(),retention_checked_at=clock_timestamp() WHERE id=$1 AND purged_at IS NULL RETURNING *)
 INSERT INTO pangu.retention_records(namespace,project_id,run_id,worker_id,status) SELECT d.namespace,d.project_id,d.run_id,p.worker_id,'purged' FROM done d JOIN pangu.projects p ON p.id=d.project_id AND p.namespace=d.namespace`, s.request)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
