package database

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestApprovalMigrationPreservesHistoryAndRemovesDeadline(t *testing.T) {
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `CREATE TEMP TABLE approval_wait(id text,status text,expires_at timestamptz,
 CONSTRAINT approvals_status_check CHECK(status IN ('waiting','approved','rejected','revoked','consumed')));
 INSERT INTO approval_wait VALUES('old','waiting',now()-interval '2 days'),
 ('current','waiting',now()+interval '1 hour'),('consumed','consumed',now()-interval '2 days'),
 ('rejected','rejected',now()-interval '2 days')`)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := source.ReadFile("migrations/0045_approval_wait.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, strings.ReplaceAll(string(sql), "pangu.approvals", "approval_wait")); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"old": "expired", "current": "waiting", "consumed": "consumed", "rejected": "rejected"} {
		var got string
		if err = conn.QueryRow(ctx, "SELECT status FROM approval_wait WHERE id=$1", id).Scan(&got); err != nil || got != want {
			t.Fatalf("迁移改变历史结果 %s: %s %v", id, got, err)
		}
	}
	var deadline bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='approval_wait'::regclass AND attname='expires_at' AND NOT attisdropped)").Scan(&deadline)
	if err != nil || deadline {
		t.Fatalf("审批期限未删除: %v", err)
	}
}

func TestStructuredReviewMigrationPreservesHistoryAndPurgesAnswers(t *testing.T) {
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	ctx := context.Background()
	conn, err := connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `CREATE SCHEMA review_migration;
 CREATE TABLE review_migration.approvals(id uuid PRIMARY KEY,run_id uuid,node_id text,namespace text,project_id uuid,
 status text CONSTRAINT approvals_status_check CHECK(status IN ('expired','consumed')),subject jsonb,reason text,
 created_at timestamptz DEFAULT now(),signalled_at timestamptz,CONSTRAINT approvals_run_id_node_id_key UNIQUE(run_id,node_id));
 CREATE INDEX approvals_outbox ON review_migration.approvals(namespace);
 INSERT INTO review_migration.approvals(id,status,subject,reason) VALUES
 ('00000000-0000-4000-8000-000000000001','expired','{"original":"完整报告"}','旧意见'),
 ('00000000-0000-4000-8000-000000000002','consumed','{"original":"执行依据"}','已核对')`)
	if err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err = tx.QueryRow(ctx, "SELECT jsonb_agg(to_jsonb(a) ORDER BY id)::text FROM review_migration.approvals a").Scan(&before); err != nil {
		t.Fatal(err)
	}
	sql, err := source.ReadFile("migrations/0046_approval_review.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, strings.ReplaceAll(string(sql), "pangu.", "review_migration.")); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT jsonb_agg(to_jsonb(a)-'round'-'previous_id'-'items' ORDER BY id)::text FROM review_migration.approvals a").Scan(&after); err != nil || before != after {
		t.Fatalf("历史内容被改写: %v", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE review_migration.approvals SET items='[{"correction":"仅文档"}]'; UPDATE review_migration.approvals SET subject=NULL`); err != nil {
		t.Fatal(err)
	}
	var valid bool
	if err = tx.QueryRow(ctx, "SELECT bool_and(round=1 AND previous_id IS NULL AND items='[]'::jsonb) FROM review_migration.approvals").Scan(&valid); err != nil || !valid {
		t.Fatalf("初始轮次或意见清理错误: %v", err)
	}
}
