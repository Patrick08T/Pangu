package database

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestSharedCatalogMigrationPreservesVersionsAndMergesExactCopies(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	conn, err := connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if conn.Config().Database != "pangu_test" {
		t.Fatal("只能在独立测试库验证迁移")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	schema := "shared_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = tx.Exec(ctx, "CREATE SCHEMA "+schema+"; CREATE TABLE "+schema+".projects(id uuid PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	applySharedMigration(t, tx, schema, "0010_catalog.sql")
	seedSharedMigration(t, tx, schema)
	applySharedMigration(t, tx, schema, "0043_shared_catalog.sql")
	applySharedMigration(t, tx, schema, "0044_catalog_duplicates.sql")
	var active, versions int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+schema+".catalog_objects WHERE kind='agent' AND NOT archived").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 2 {
		t.Fatalf("应保留原角色和不同配置，实际 %d 个", active)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+schema+".catalog_versions").Scan(&versions); err != nil || versions != 5 {
		t.Fatalf("历史版本被删除: %d %v", versions, err)
	}
	var draft, published string
	if err = tx.QueryRow(ctx, "SELECT spec#>>'{flow,nodes,0,agent_version}' FROM "+schema+".catalog_objects WHERE kind='workflow'").Scan(&draft); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT v.spec#>>'{flow,nodes,0,agent_version}' FROM "+schema+".catalog_versions v JOIN "+schema+".catalog_objects o ON o.id=v.object_id WHERE o.kind='workflow'").Scan(&published); err != nil {
		t.Fatal(err)
	}
	if draft != "00000000-0000-0000-0000-000000000011" || published != "00000000-0000-0000-0000-000000000012" {
		t.Fatalf("草稿未更新或历史引用被改写: %s %s", draft, published)
	}
	if _, err = tx.Exec(ctx, "UPDATE "+schema+".catalog_versions SET spec='{}'"); err == nil {
		t.Fatal("迁移关闭了历史保护")
	}
}

func applySharedMigration(t *testing.T, tx pgx.Tx, schema, name string) {
	t.Helper()
	data, err := source.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), strings.ReplaceAll(string(data), "pangu.", schema+".")); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func seedSharedMigration(t *testing.T, tx pgx.Tx, schema string) {
	t.Helper()
	query := `INSERT INTO pangu.projects VALUES('00000000-0000-0000-0000-000000000001');
INSERT INTO pangu.catalog_objects(id,namespace,kind,project_id,name,spec,created_at) VALUES
('00000000-0000-0000-0000-000000000002','test','template',NULL,'角色','{"agent":{"id":"role","instruction":"同一配置"}}','2026-01-01'),
('00000000-0000-0000-0000-000000000003','test','agent','00000000-0000-0000-0000-000000000001','角色','{"agent":{"id":"role","instruction":"同一配置"},"template_version":"00000000-0000-0000-0000-000000000011"}','2026-01-02'),
('00000000-0000-0000-0000-000000000004','test','agent','00000000-0000-0000-0000-000000000001','角色','{"agent":{"id":"role","instruction":"不同配置"}}','2026-01-03'),
('00000000-0000-0000-0000-000000000005','test','workflow','00000000-0000-0000-0000-000000000001','流程','{"flow":{"nodes":[{"id":"check","agent_version":"00000000-0000-0000-0000-000000000012"}]}}','2026-01-04');
INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) SELECT
CASE id::text WHEN '00000000-0000-0000-0000-000000000002' THEN '00000000-0000-0000-0000-000000000011' WHEN '00000000-0000-0000-0000-000000000003' THEN '00000000-0000-0000-0000-000000000012' WHEN '00000000-0000-0000-0000-000000000004' THEN '00000000-0000-0000-0000-000000000013' ELSE '00000000-0000-0000-0000-000000000014' END::uuid,id,1,1,spec FROM pangu.catalog_objects;
INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) VALUES('00000000-0000-0000-0000-000000000015','00000000-0000-0000-0000-000000000003',2,2,'{"agent":{"id":"role","instruction":"同一配置"}}');`
	if _, err := tx.Exec(context.Background(), strings.ReplaceAll(query, "pangu.", schema+".")); err != nil {
		t.Fatal(err)
	}
}
